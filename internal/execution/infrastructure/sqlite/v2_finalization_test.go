package sqlite

import (
	"context"
	"errors"
	d "github.com/javleds/api-engine/internal/execution/domain"
	"testing"
	"time"
)

func TestV2TerminalAndOutboxCommitTogether(t *testing.T) {
	store, db := testStore(t)
	ctx := context.Background()
	snapshot := testSnapshot("v2-terminal", true)
	snapshot.APIVersion = 2
	snapshot.DeadlineAt = time.Now().Add(time.Hour)
	if _, _, err := store.Create(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.Claim(ctx, snapshot.ID, "instance", 0, time.Now()); err != nil || !ok {
		t.Fatalf("claim %v", err)
	}
	c := d.ExecutionCompletion{APIVersion: 2, ID: snapshot.ID, ExpectedVersion: 1, Status: d.Completed, Report: d.EmptyReport(), FinishedAt: time.Now(), ResultExpiresAt: time.Now().Add(time.Hour), LastSequence: "1", Delivery: &d.WebhookRecord{ID: "delivery-v2", EventID: "event-v2", Target: []byte("target"), Payload: []byte(`{"lastSequence":"1"}`), DeadlineAt: time.Now().Add(time.Hour)}}
	c.Report.Roots = []d.RootOutcome{{Name: "items", Status: d.Completed, DataAvailable: true}}
	finalizer := NewExecutionFinalizer(store)
	if _, err := db.Exec(`CREATE TRIGGER reject_v2_delivery BEFORE INSERT ON webhook_deliveries BEGIN SELECT RAISE(ABORT,'simulated disk failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := finalizer.Finish(ctx, c); err == nil {
		t.Fatal("expected outbox failure")
	}
	state, err := store.ReadV2State(ctx, c.ID)
	if err != nil || state.Terminal || state.LastSequence != "0" {
		t.Fatalf("partial publication %+v %v", state, err)
	}
	current, err := store.Read(ctx, "owner", c.ID)
	if err != nil || current.Status != d.Running {
		t.Fatalf("partial terminal %+v %v", current, err)
	}
	if _, err = db.Exec(`DROP TRIGGER reject_v2_delivery`); err != nil {
		t.Fatal(err)
	}
	if err = finalizer.Finish(ctx, c); err != nil {
		t.Fatal(err)
	}
	page, err := store.ReadV2Chunks(ctx, "owner", c.ID, 0, 100, 1<<20, time.Now())
	if err != nil || !page.Terminal || len(page.Chunks) != 1 || page.LastSequence != "1" {
		t.Fatalf("terminal %+v %v", page, err)
	}
	if err = finalizer.Finish(ctx, c); !errors.Is(err, d.ErrVersionConflict) {
		t.Fatalf("duplicate terminal %v", err)
	}
	if _, err = store.ReadV2Chunks(ctx, "owner", c.ID, 0, 100, 1<<20, time.Now().Add(2*time.Hour)); !errors.Is(err, d.ErrResultExpired) {
		t.Fatalf("expiry %v", err)
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM execution_results WHERE execution_id=?`, c.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("v1 result unexpectedly assembled %d %v", count, err)
	}
}
