package sqlite

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	d "github.com/javleds/api-engine/internal/execution/domain"
	"strings"
	"testing"
	"time"
)

func TestV2ResultEventAndCheckpointAreAtomic(t *testing.T) {
	s, db := testStore(t)
	ctx := context.Background()
	snapshot := seedV2(t, s, "result-atomic")
	task, err := s.NextV2Task(ctx, snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	c := d.V2Commit{ID: snapshot.ID, OwnerID: "owner", Generation: 1, Task: task, Chunks: []d.V2Chunk{{Type: "data", Operation: "value", NodeID: "node", Value: json.RawMessage(`9007199254740993`)}}, Results: []d.V2Result{{NodeID: "node", Root: "secret-root", Path: "/secret-root", Status: "completed", FirstChunk: 1, LastChunk: 1}}}
	if _, err = db.Exec(`CREATE TRIGGER reject_result BEFORE INSERT ON v2_results BEGIN SELECT RAISE(ABORT,'simulated result failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err = s.CommitV2Task(ctx, c); err == nil {
		t.Fatal("expected atomic transaction failure")
	}
	state, _ := s.ReadV2State(ctx, snapshot.ID)
	if state.LastSequence != "0" || state.ResultCount != 0 {
		t.Fatalf("partial publication %+v", state)
	}
	saved, _ := s.NextV2Task(ctx, snapshot.ID)
	if saved.Revision != task.Revision {
		t.Fatal("checkpoint escaped rollback")
	}
	if _, err = db.Exec(`DROP TRIGGER reject_result`); err != nil {
		t.Fatal(err)
	}
	if err = s.CommitV2Task(ctx, c); err != nil {
		t.Fatal(err)
	}
	if err = s.CommitV2Task(ctx, c); !errors.Is(err, d.ErrVersionConflict) {
		t.Fatalf("duplicate task accepted: %v", err)
	}
	page, err := s.ReadV2Results(ctx, "owner", snapshot.ID, 0, 0, 100, time.Now())
	if err != nil || len(page.Results) != 1 || page.Results[0].EventID == "" || page.NextCursor != "1" {
		t.Fatalf("page %+v %v", page, err)
	}
	var blob []byte
	if err = db.QueryRow(`SELECT payload_blob FROM v2_results WHERE execution_id=?`, snapshot.ID).Scan(&blob); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, []byte("secret-root")) {
		t.Fatal("unencrypted result metadata")
	}
	beforeID := page.Results[0].EventID
	if err = s.RecoverV2(ctx); err != nil {
		t.Fatal(err)
	}
	page, err = s.ReadV2Results(ctx, "owner", snapshot.ID, 0, 0, 100, time.Now())
	if err != nil || page.Results[0].EventID != beforeID {
		t.Fatal("event identity changed on recovery")
	}
	if _, err = s.ReadV2Result(ctx, "other", snapshot.ID, "node", time.Now()); !errors.Is(err, d.ErrNotFound) {
		t.Fatalf("owner leaked: %v", err)
	}
	if _, err = db.Exec(`UPDATE executions SET status='completed',result_expires_at=? WHERE id=?`, time.Now().Add(-time.Second).UnixMilli(), snapshot.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReadV2Result(ctx, "owner", snapshot.ID, "node", time.Now()); !errors.Is(err, d.ErrResultExpired) {
		t.Fatalf("result expiry: %v", err)
	}
	if err = s.PurgeV2(ctx, time.Now(), 100); err != nil {
		t.Fatal(err)
	}
	state, err = s.ReadV2State(ctx, snapshot.ID)
	if err != nil || state.StoredBytes != 0 {
		t.Fatalf("result accounting not purged: %+v %v", state, err)
	}
}

func TestV2LegacyResultsUnavailable(t *testing.T) {
	s, db := testStore(t)
	snapshot := seedV2(t, s, "legacy-results")
	if _, err := db.Exec(`UPDATE v2_state SET results_enabled=0 WHERE execution_id=?`, snapshot.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadV2Results(context.Background(), "owner", snapshot.ID, 0, 0, 100, time.Now()); !errors.Is(err, d.ErrV2ResultsUnavailable) {
		t.Fatalf("legacy capability: %v", err)
	}
}

func TestV2ResultStreamingLargerThanChunkLimit(t *testing.T) {
	s, _ := testStore(t)
	ctx := context.Background()
	snapshot := seedV2(t, s, "large-chain")
	task, _ := s.NextV2Task(ctx, snapshot.ID)
	c := d.V2Commit{ID: snapshot.ID, OwnerID: "owner", Generation: 1, Task: task, Chunks: []d.V2Chunk{{Type: "data", Operation: "object", NodeID: "root"}}}
	for _, key := range []string{"a", "b", "c"} {
		value, _ := json.Marshal(strings.Repeat(key, 128<<10))
		c.Chunks = append(c.Chunks, d.V2Chunk{Type: "data", Operation: "value", NodeID: key, ParentID: "root", Key: key, Value: value})
	}
	c.Chunks = append(c.Chunks, d.V2Chunk{Type: "data", Operation: "close", NodeID: "root", Status: "completed"})
	c.Results = []d.V2Result{{NodeID: "root", Root: "root", Status: "completed", FirstChunk: 1, LastChunk: 5}}
	if err := s.CommitV2Task(ctx, c); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	if err := s.WriteV2Result(ctx, "owner", snapshot.ID, "root", time.Now(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Len() < 384<<10 || !json.Valid(body.Bytes()) || !strings.HasSuffix(body.String(), `"complete":true}`) {
		t.Fatalf("invalid large result length=%d", body.Len())
	}
}
