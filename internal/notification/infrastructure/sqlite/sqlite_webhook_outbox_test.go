package sqlite

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	executiondomain "github.com/javleds/api-engine/internal/execution/domain"
	executionstore "github.com/javleds/api-engine/internal/execution/infrastructure/sqlite"
	dbplatform "github.com/javleds/api-engine/internal/platform/sqlite"
	security "github.com/javleds/api-engine/internal/security/infrastructure"
)

func testOutbox(t *testing.T) (*SQLiteWebhookOutbox, time.Time) {
	t.Helper()
	ctx := context.Background()
	db, err := dbplatform.Open(ctx, filepath.Join(t.TempDir(), "engine.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := dbplatform.Migrate(ctx, db, "engine"); err != nil {
		t.Fatal(err)
	}
	protector, err := security.NewAESGCMProtector("test", map[string][]byte{"test": bytes.Repeat([]byte{3}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	store := executionstore.NewExecutionStore(db, protector)
	now := time.Now().UTC().Truncate(time.Millisecond)
	snapshot := executiondomain.ExecutionSnapshot{ID: "execution", OwnerID: "owner", Definition: []byte(`{}`), WebhookEnabled: true, CreatedAt: now, MetadataExpiresAt: now.Add(time.Hour)}
	if _, _, err := store.Create(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	completion := executiondomain.ExecutionCompletion{ID: snapshot.ID, Status: executiondomain.Failed, Error: &executiondomain.ExecutionFailure{Code: "PROCESS_INTERRUPTED", Message: "Interrumpido"}, Report: executiondomain.EmptyReport(), FinishedAt: now, Delivery: &executiondomain.WebhookRecord{ID: "delivery", EventID: "stable-event", Target: []byte(`{"url":"https://receiver.example"}`), Payload: []byte(`{"eventId":"stable-event"}`), DeadlineAt: now.Add(time.Hour)}}
	if err := executionstore.NewExecutionFinalizer(store).Finish(ctx, completion); err != nil {
		t.Fatal(err)
	}
	return NewSQLiteWebhookOutbox(db, protector), now
}

func TestOutboxLeaseRetryAndRecovery(t *testing.T) {
	outbox, now := testOutbox(t)
	ctx := context.Background()
	first, err := outbox.Claim(ctx, "old", now, time.Minute)
	if err != nil || first == nil {
		t.Fatalf("claim: %v", err)
	}
	if first.Attempts != 1 || string(first.Payload) != `{"eventId":"stable-event"}` {
		t.Fatalf("first: %+v", first)
	}
	if next, err := outbox.Claim(ctx, "other", now, time.Minute); err != nil || next != nil {
		t.Fatalf("double claim: %+v %v", next, err)
	}
	if err := outbox.Recover(ctx, "new", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	second, err := outbox.Claim(ctx, "new", now.Add(time.Second), time.Minute)
	if err != nil || second == nil {
		t.Fatalf("recovery claim: %v", err)
	}
	if second.EventID != first.EventID || second.Attempts != 2 || !bytes.Equal(first.Payload, second.Payload) {
		t.Fatal("event changed during recovery")
	}
	if err := outbox.SaveAttempt(ctx, *first, true, nil, 200, "", now.Add(time.Second)); !errors.Is(err, executiondomain.ErrVersionConflict) {
		t.Fatalf("stale lease: %v", err)
	}
	retryAt := now.Add(time.Minute)
	if err := outbox.SaveAttempt(ctx, *second, false, &retryAt, 503, "Servicio temporalmente no disponible.", now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if next, err := outbox.Claim(ctx, "new", now.Add(3*time.Second), time.Minute); err != nil || next != nil {
		t.Fatalf("early retry: %v", err)
	}
	third, err := outbox.Claim(ctx, "new", retryAt, time.Minute)
	if err != nil || third == nil {
		t.Fatalf("retry claim: %v", err)
	}
	if err := outbox.SaveAttempt(ctx, *third, true, nil, 204, "", retryAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	status, err := outbox.Status(ctx, "owner", "execution")
	if err != nil || status.Status != "delivered" || status.Attempts != 3 {
		t.Fatalf("status %+v %v", status, err)
	}
	if status, err := outbox.Status(ctx, "someone-else", "execution"); err != nil || status != nil {
		t.Fatal("owner leak")
	}
}

func TestOutboxExpiredDeadline(t *testing.T) {
	outbox, now := testOutbox(t)
	ctx := context.Background()
	delivery, err := outbox.Claim(ctx, "instance", now.Add(2*time.Hour), time.Minute)
	if err != nil || delivery != nil {
		t.Fatalf("expired claim: %v", err)
	}
	status, err := outbox.Status(ctx, "owner", "execution")
	if err != nil || status.Status != "failed" {
		t.Fatalf("expired status: %+v %v", status, err)
	}
}
