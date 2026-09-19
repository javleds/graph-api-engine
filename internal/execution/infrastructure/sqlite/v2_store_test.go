package sqlite

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	d "github.com/javleds/api-engine/internal/execution/domain"
)

func seedV2(t *testing.T, s *ExecutionStore, id string) d.ExecutionSnapshot {
	t.Helper()
	snapshot := testSnapshot(id, false)
	snapshot.APIVersion = 2
	snapshot.IdempotencyKeyHash = id
	snapshot.DeadlineAt = snapshot.CreatedAt.Add(time.Hour)
	if _, _, err := s.Create(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedV2Tasks(context.Background(), snapshot.ID, snapshot.OwnerID, 1, []d.V2Task{{Key: "coordinator", Payload: json.RawMessage(`{"phase":"fetch"}`)}}); err != nil {
		t.Fatal(err)
	}
	return snapshot
}
func TestV2AtomicCheckpointReplayAndEncryption(t *testing.T) {
	s, db := testStore(t)
	ctx := context.Background()
	snapshot := seedV2(t, s, "v2-one")
	task, err := s.NextV2Task(ctx, snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	task.Payload = json.RawMessage(`{"phase":"publish"}`)
	commit := d.V2Commit{ID: snapshot.ID, OwnerID: snapshot.OwnerID, Generation: 1, Task: task, Inputs: []d.V2Input{{Key: "page", Payload: json.RawMessage(`{"secret":"provider-secret"}`)}}, Chunks: []d.V2Chunk{{Type: "data", Operation: "object", NodeID: "root"}, {Type: "data", Operation: "value", NodeID: "value", ParentID: "root", Key: "secret", Value: json.RawMessage(`"provider-secret"`)}}}
	if err := s.CommitV2Task(ctx, commit); err != nil {
		t.Fatal(err)
	}
	if err := s.CommitV2Task(ctx, commit); !errors.Is(err, d.ErrVersionConflict) {
		t.Fatalf("duplicate checkpoint: %v", err)
	}
	page, err := s.ReadV2Chunks(ctx, "owner", snapshot.ID, 0, 100, 1<<20, time.Now())
	if err != nil || len(page.Chunks) != 2 || page.NextCursor != "2" {
		t.Fatalf("chunks %#v %v", page, err)
	}
	if _, err = s.ReadV2Chunks(ctx, "stranger", snapshot.ID, 0, 100, 1<<20, time.Now()); !errors.Is(err, d.ErrNotFound) {
		t.Fatalf("owner isolation: %v", err)
	}
	input, err := s.ReadV2Input(ctx, snapshot.ID, "page")
	if err != nil || !bytes.Contains(input, []byte("provider-secret")) {
		t.Fatalf("input: %s %v", input, err)
	}
	for _, table := range []string{"v2_inputs", "v2_chunks", "v2_tasks", "v2_nodes"} {
		rows, err := db.Query("SELECT payload_blob FROM " + table)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var b []byte
			if err = rows.Scan(&b); err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(b, []byte("provider-secret")) {
				t.Fatalf("plaintext in %s", table)
			}
		}
		rows.Close()
	}
	state, err := s.ReadV2State(ctx, snapshot.ID)
	if err != nil || state.StoredBytes <= state.PublishedBytes || state.LastSequence != "2" {
		t.Fatalf("state %#v %v", state, err)
	}
}
func TestV2RecoveryBudgetsAndLimitRollback(t *testing.T) {
	s, _ := testStore(t)
	ctx := context.Background()
	snapshot := seedV2(t, s, "v2-recovery")
	if err := s.ReserveV2Budget(ctx, snapshot.ID, 1, "attempt", 1); err != nil {
		t.Fatal(err)
	}
	if err := s.RecoverV2(ctx); err != nil {
		t.Fatal(err)
	}
	state, err := s.ReadV2State(ctx, snapshot.ID)
	if err != nil || state.Generation != 2 || state.Recoveries != 1 || state.HTTPAttempts != 1 || !state.DeadlineAt.Equal(time.UnixMilli(snapshot.DeadlineAt.UnixMilli())) {
		t.Fatalf("state %#v %v", state, err)
	}
	if err := s.ReserveV2Budget(ctx, snapshot.ID, 2, "attempt", 1); !errors.Is(err, d.ErrV2Limit) {
		t.Fatalf("budget reset: %v", err)
	}
	task, err := s.NextV2Task(ctx, snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	task.Done = true
	c := d.V2Commit{ID: snapshot.ID, OwnerID: "owner", Generation: 1, Task: task}
	if err := s.CommitV2Task(ctx, c); !errors.Is(err, d.ErrVersionConflict) {
		t.Fatalf("stale generation: %v", err)
	}
	c.Generation = 2
	c.Chunks = []d.V2Chunk{{Type: "data", Operation: "value", NodeID: "root", Value: json.RawMessage(`"` + strings.Repeat("x", 256<<10) + `"`)}}
	if err := s.CommitV2Task(ctx, c); !errors.Is(err, d.ErrV2ValueTooLarge) {
		t.Fatalf("oversized scalar: %v", err)
	}
	restored, err := s.NextV2Task(ctx, snapshot.ID)
	if err != nil || restored.Done || restored.Revision != 0 {
		t.Fatalf("checkpoint escaped rollback: %#v %v", restored, err)
	}
	c.Chunks = []d.V2Chunk{{Type: "data", Operation: "value", NodeID: "root", Value: json.RawMessage(`1`)}}
	c.Limits = d.DefaultV2Limits()
	c.Limits.MaxStoredBytes = 1
	if err := s.CommitV2Task(ctx, c); !errors.Is(err, d.ErrV2Limit) {
		t.Fatalf("storage limit: %v", err)
	}
	state, _ = s.ReadV2State(ctx, snapshot.ID)
	if state.LastSequence != "0" {
		t.Fatal("failed commit published")
	}
}
func TestV2TreePaginationClosureAndExpiry(t *testing.T) {
	s, db := testStore(t)
	ctx := context.Background()
	snapshot := seedV2(t, s, "v2-tree")
	task, _ := s.NextV2Task(ctx, snapshot.ID)
	zero, one := int64(0), int64(1)
	c := d.V2Commit{ID: snapshot.ID, OwnerID: "owner", Generation: 1, Task: task, Chunks: []d.V2Chunk{{Type: "data", Operation: "array", NodeID: "root"}, {Type: "data", Operation: "value", NodeID: "a", ParentID: "root", Index: &zero, Value: json.RawMessage(`9007199254740993`)}, {Type: "data", Operation: "value", NodeID: "b", ParentID: "root", Index: &one, Value: json.RawMessage(`null`)}, {Type: "data", Operation: "close", NodeID: "root", Status: "completed"}}}
	if err := s.CommitV2Task(ctx, c); err != nil {
		t.Fatal(err)
	}
	first, err := s.ReadV2Tree(ctx, "owner", snapshot.ID, "root", 0, 1, time.Now())
	if err != nil || len(first.Nodes) != 1 || first.Nodes[0].NodeID != "a" || first.NextCursor != "1" || !first.HasMore {
		t.Fatalf("first %#v %v", first, err)
	}
	next, err := s.ReadV2Tree(ctx, "owner", snapshot.ID, "root", 1, 1, time.Now())
	if err != nil || len(next.Nodes) != 1 || next.Nodes[0].NodeID != "b" || next.HasMore {
		t.Fatalf("next %#v %v", next, err)
	}
	root, err := s.ReadV2Tree(ctx, "owner", snapshot.ID, "", 0, 100, time.Now())
	if err != nil || root.Nodes[0].Status != "completed" {
		t.Fatalf("root %#v %v", root, err)
	}
	if _, err := db.Exec(`UPDATE executions SET status='failed',result_expires_at=? WHERE id=?`, time.Now().Add(-time.Second).UnixMilli(), snapshot.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadV2Chunks(ctx, "owner", snapshot.ID, 0, 100, 1<<20, time.Now()); !errors.Is(err, d.ErrResultExpired) {
		t.Fatalf("expiry %v", err)
	}
	if err := s.PurgeV2(ctx, time.Now(), 100); err != nil {
		t.Fatal(err)
	}
	state, err := s.ReadV2State(ctx, snapshot.ID)
	if err != nil || state.StoredBytes != 0 {
		t.Fatalf("purged accounting %#v %v", state, err)
	}
}

func TestV2TerminalPublishesFailureAtomicallyAndRejectsLateWrites(t *testing.T) {
	s, _ := testStore(t)
	ctx := context.Background()
	snapshot := seedV2(t, s, "v2-terminal")
	task, _ := s.NextV2Task(ctx, snapshot.ID)
	finish := d.ExecutionCompletion{APIVersion: 2, ID: snapshot.ID, ExpectedVersion: 0, LastSequence: "2", Status: d.Failed, Error: &d.ExecutionFailure{Code: "RESULT_VALUE_TOO_LARGE", Message: "scalar too large"}, Report: d.EmptyReport(), FinishedAt: time.Now(), ResultExpiresAt: time.Now().Add(time.Hour)}
	finalizer := NewExecutionFinalizer(s)
	if err := finalizer.Finish(ctx, finish); !errors.Is(err, d.ErrVersionConflict) {
		t.Fatalf("terminal sequence conflict: %v", err)
	}
	execution, _ := s.Read(ctx, "owner", snapshot.ID)
	if execution.Status != d.Pending {
		t.Fatal("failed terminal transaction changed status")
	}
	finish.LastSequence = "1"
	if err := finalizer.Finish(ctx, finish); err != nil {
		t.Fatal(err)
	}
	page, err := s.ReadV2Chunks(ctx, "owner", snapshot.ID, 0, 100, 1<<20, time.Now())
	if err != nil || !page.Terminal || len(page.Chunks) != 1 || !bytes.Contains(page.Chunks[0], []byte("RESULT_VALUE_TOO_LARGE")) {
		t.Fatalf("terminal %#v %v", page, err)
	}
	if err := s.CommitV2Task(ctx, d.V2Commit{ID: snapshot.ID, OwnerID: "owner", Generation: 1, Task: task}); !errors.Is(err, d.ErrVersionConflict) {
		t.Fatalf("late publish: %v", err)
	}
}
