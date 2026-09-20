package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/javleds/api-engine/internal/notification/domain"
)

func TestNodeOutboxAtomicCursorCapacityAndReplay(t *testing.T) {
	outbox, now := testOutbox(t)
	ctx := context.Background()
	if _, err := outbox.db.Exec(`INSERT INTO v2_state(execution_id,deadline_at,results_enabled,last_result_sequence) VALUES('execution',?,1,1)`, now.Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	c := domain.NodeEventCursor{ExecutionID: "execution", OwnerID: "owner"}
	d := domain.PreparedNodeDelivery{WebhookDelivery: domain.WebhookDelivery{ID: "node-delivery", EventID: "node-event", ExecutionID: c.ExecutionID, OwnerID: c.OwnerID, Target: []byte(`{"url":"https://receiver.example"}`), Payload: []byte(`{"result":{"n":9007199254740993}}`), DeadlineAt: now.Add(time.Hour), NextAttemptAt: now}, Sequence: 1, OccurredAt: now}
	ok, err := outbox.SaveNodeDelivery(ctx, c, d, 1, 1<<20)
	if err != nil || ok {
		t.Fatalf("capacity: %v %v", ok, err)
	}
	var cursor int64
	if err = outbox.db.QueryRow(`SELECT webhook_cursor FROM v2_state WHERE execution_id='execution'`).Scan(&cursor); err != nil || cursor != 0 {
		t.Fatal(cursor, err)
	}
	if _, err = outbox.db.Exec(`CREATE TRIGGER fail_cursor BEFORE UPDATE OF webhook_cursor ON v2_state BEGIN SELECT RAISE(ABORT,'simulated failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = outbox.SaveNodeDelivery(ctx, c, d, 2, 1<<20); err == nil {
		t.Fatal("expected transaction failure")
	}
	var count int
	if err = outbox.db.QueryRow(`SELECT count(*) FROM webhook_deliveries WHERE event_type='node.finished'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("partial commit", count, err)
	}
	if _, err = outbox.db.Exec(`DROP TRIGGER fail_cursor`); err != nil {
		t.Fatal(err)
	}
	if ok, err = outbox.SaveNodeDelivery(ctx, c, d, 2, 1<<20); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if ok, err = outbox.SaveNodeDelivery(ctx, c, d, 2, 1<<20); err != nil || !ok {
		t.Fatal("replay", ok, err)
	}
	status, err := outbox.Status(ctx, "owner", "execution")
	if err != nil || status == nil || status.Status != "pending" {
		t.Fatal("terminal status", status, err)
	}
	summary, err := outbox.NodeSummary(ctx, "owner", "execution")
	if err != nil || summary.Pending != 1 {
		t.Fatal("summary", summary, err)
	}
	if summary, err = outbox.NodeSummary(ctx, "other", "execution"); err != nil || summary.Pending != 0 {
		t.Fatal("owner leak", summary, err)
	}
}

func TestFailedNodeDeliveryBypassesFullQueueWithoutHTTP(t *testing.T) {
	outbox, now := testOutbox(t)
	ctx := context.Background()
	if _, err := outbox.db.Exec(`INSERT INTO v2_state(execution_id,deadline_at,results_enabled,last_result_sequence) VALUES('execution',?,1,1)`, now.Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	c := domain.NodeEventCursor{ExecutionID: "execution", OwnerID: "owner"}
	d := domain.PreparedNodeDelivery{WebhookDelivery: domain.WebhookDelivery{ID: "oversize", EventID: "node-event", ExecutionID: c.ExecutionID, OwnerID: c.OwnerID, Target: []byte(`{}`), DeadlineAt: now.Add(time.Hour), NextAttemptAt: now}, Sequence: 1, OccurredAt: now, Failure: "WEBHOOK_PAYLOAD_TOO_LARGE"}
	if ok, err := outbox.SaveNodeDelivery(ctx, c, d, 1, 1); err != nil || ok {
		t.Fatal("failed record exceeded storage budget", ok, err)
	}
	if ok, err := outbox.SaveNodeDelivery(ctx, c, d, 1, 1<<20); err != nil || !ok {
		t.Fatal(ok, err)
	}
	var status string
	var attempts int
	if err := outbox.db.QueryRow(`SELECT status,attempts FROM webhook_deliveries WHERE id='oversize'`).Scan(&status, &attempts); err != nil || status != "failed" || attempts != 0 {
		t.Fatal(status, attempts, err)
	}
	summary, err := outbox.NodeSummary(ctx, "owner", "execution")
	if err != nil || summary.Failed != 1 || summary.Pending != 0 {
		t.Fatal(summary, err)
	}
}
