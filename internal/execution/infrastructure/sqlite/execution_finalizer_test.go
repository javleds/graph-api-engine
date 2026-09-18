package sqlite

import (
	"context"
	"errors"
	d "github.com/javleds/api-engine/internal/execution/domain"
	"testing"
	"time"
)

func TestAtomicFinalizationAndExpiration(t *testing.T) {
	store, db := testStore(t)
	ctx := context.Background()
	snapshot := testSnapshot("one", true)
	if _, _, err := store.Create(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	c := d.ExecutionCompletion{ID: snapshot.ID, ExpectedVersion: 0, Status: d.Completed, Report: d.EmptyReport(), Data: []byte(`{"items":[]}`), FinishedAt: time.Now(), ResultExpiresAt: time.Now().Add(time.Hour), Delivery: &d.WebhookRecord{ID: "delivery", EventID: "event", Target: []byte("secret-target"), Payload: []byte(`{"eventId":"event"}`), DeadlineAt: time.Now().Add(time.Hour)}}
	finalizer := NewExecutionFinalizer(store)
	if _, err := db.Exec(`CREATE TRIGGER reject_delivery BEFORE INSERT ON webhook_deliveries BEGIN SELECT RAISE(ABORT,'simulated disk failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := finalizer.Finish(ctx, c); err == nil {
		t.Fatal("injected failure not surfaced")
	}
	execution, err := store.Read(ctx, "owner", snapshot.ID)
	if err != nil || execution.Status != d.Pending {
		t.Fatalf("partial transition: %v %v", execution.Status, err)
	}
	var results int
	db.QueryRow("SELECT count(*) FROM execution_results").Scan(&results)
	if results != 0 {
		t.Fatal("orphan result")
	}
	if _, err := db.Exec("DROP TRIGGER reject_delivery"); err != nil {
		t.Fatal(err)
	}
	if err := finalizer.Finish(ctx, c); err != nil {
		t.Fatal(err)
	}
	execution, data, err := store.ReadResult(ctx, "owner", snapshot.ID, time.Now())
	if err != nil || string(data) != string(c.Data) || execution.Status != d.Completed {
		t.Fatalf("result: %s %v", data, err)
	}
	if err := finalizer.Finish(ctx, c); !errors.Is(err, d.ErrVersionConflict) {
		t.Fatalf("immutable terminal: %v", err)
	}
	if _, _, err := store.ReadResult(ctx, "owner", snapshot.ID, time.Now().Add(2*time.Hour)); !errors.Is(err, d.ErrResultExpired) {
		t.Fatalf("expiry: %v", err)
	}
}
