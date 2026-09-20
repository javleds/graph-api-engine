package sqlite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/javleds/api-engine/internal/notification/domain"
)

func orderedOutbox(t *testing.T, ordering string, window int) (*SQLiteWebhookOutbox, time.Time) {
	t.Helper()
	o, now := testOutbox(t)
	if _, err := o.db.Exec(`UPDATE executions SET api_version=2,webhook_ordering=?,webhook_max_in_flight=? WHERE id='execution'`, ordering, window); err != nil {
		t.Fatal(err)
	}
	if _, err := o.db.Exec(`INSERT INTO v2_state(execution_id,deadline_at,results_enabled) VALUES('execution',?,1)`, now.Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	return o, now
}

func addOrderedResult(t *testing.T, o *SQLiteWebhookOutbox, now time.Time, sequence int64, root, index int) domain.PreparedNodeDelivery {
	t.Helper()
	id := fmt.Sprintf("node-%d", sequence)
	if _, err := o.db.Exec(`INSERT INTO v2_results(execution_id,node_id,sequence,event_id,payload_blob,plaintext_size,first_chunk,last_chunk,occurred_at,source_root,source_index) VALUES('execution',?,?,?,X'00',1,1,1,?,?,?)`, id, sequence, id, now.UnixMilli(), root, index); err != nil {
		t.Fatal(err)
	}
	if _, err := o.db.Exec(`UPDATE v2_state SET last_result_sequence=? WHERE execution_id='execution'`, sequence); err != nil {
		t.Fatal(err)
	}
	return domain.PreparedNodeDelivery{WebhookDelivery: domain.WebhookDelivery{ID: id, EventID: id, ExecutionID: "execution", OwnerID: "owner", Target: []byte(`{}`), Payload: []byte(`{}`), DeadlineAt: now.Add(time.Hour), NextAttemptAt: now}, Sequence: sequence, OccurredAt: now}
}

func projectOrderedResult(t *testing.T, o *SQLiteWebhookOutbox, d domain.PreparedNodeDelivery, source bool, maximum int) bool {
	t.Helper()
	ok, err := o.SaveNodeDelivery(context.Background(), domain.NodeEventCursor{ExecutionID: "execution", OwnerID: "owner", After: d.Sequence - 1, Source: source}, d, maximum, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func claimOrdered(t *testing.T, o *SQLiteWebhookOutbox, now time.Time, want string) *domain.WebhookDelivery {
	t.Helper()
	d, err := o.Claim(context.Background(), "instance", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if want == "" {
		if d != nil {
			t.Fatalf("unexpected delivery %s", d.ID)
		}
		return nil
	}
	if d == nil || d.ID != want {
		t.Fatalf("want %s, got %+v", want, d)
	}
	return d
}

func TestCompletionWindowRetainsRetriesAndGatesTerminal(t *testing.T) {
	ctx := context.Background()
	o, now := orderedOutbox(t, "completion", 1)
	for sequence := int64(1); sequence <= 2; sequence++ {
		projectOrderedResult(t, o, addOrderedResult(t, o, now, sequence, 0, int(sequence-1)), false, 10)
	}
	first := claimOrdered(t, o, now, "node-1")
	claimOrdered(t, o, now, "")
	retryAt := now.Add(20 * time.Second)
	if err := o.SaveAttempt(ctx, *first, false, &retryAt, 503, "UNAVAILABLE", now); err != nil {
		t.Fatal(err)
	}
	claimOrdered(t, o, now.Add(time.Second), "")
	first = claimOrdered(t, o, retryAt, "node-1")
	if err := o.SaveAttempt(ctx, *first, true, nil, 202, "", retryAt); err != nil {
		t.Fatal(err)
	}
	second := claimOrdered(t, o, retryAt, "node-2")
	if err := o.SaveAttempt(ctx, *second, false, nil, 400, "REJECTED", retryAt); err != nil {
		t.Fatal(err)
	}
	claimOrdered(t, o, retryAt, "")
	if _, err := o.db.Exec(`UPDATE v2_state SET terminal=1 WHERE execution_id='execution'`); err != nil {
		t.Fatal(err)
	}
	claimOrdered(t, o, retryAt, "delivery")
}

func TestCompletionWindowAllowsConcurrentDeliveryAndOperatorCap(t *testing.T) {
	o, now := orderedOutbox(t, "completion", 100)
	o.maxConcurrent = 2
	for sequence := int64(1); sequence <= 3; sequence++ {
		projectOrderedResult(t, o, addOrderedResult(t, o, now, sequence, 0, int(sequence-1)), false, 10)
	}
	claimOrdered(t, o, now, "node-1")
	claimOrdered(t, o, now, "node-2")
	claimOrdered(t, o, now, "")
}

func TestSourceProjectionSkipsCompletionOrderWithoutCapacityDeadlock(t *testing.T) {
	ctx := context.Background()
	o, now := orderedOutbox(t, "source", 2)
	// This fixture already has a terminal record; omit it while exercising a
	// running execution with exactly one available node-delivery slot.
	if _, err := o.db.Exec(`DELETE FROM webhook_deliveries WHERE id='delivery'`); err != nil {
		t.Fatal(err)
	}
	later := addOrderedResult(t, o, now, 1, 1, -1)
	cursor := domain.NodeEventCursor{ExecutionID: "execution", OwnerID: "owner", Source: true}
	if sequence, err := o.NextSourceEvent(ctx, cursor); err != nil || sequence != 0 {
		t.Fatalf("later root must wait: %d %v", sequence, err)
	}
	earlier := addOrderedResult(t, o, now, 2, 0, -1)
	if _, err := o.db.Exec(`UPDATE v2_state SET source_root=1 WHERE execution_id='execution'`); err != nil {
		t.Fatal(err)
	}
	if sequence, err := o.NextSourceEvent(ctx, cursor); err != nil || sequence != 2 {
		t.Fatalf("earlier source first: %d %v", sequence, err)
	}
	if !projectOrderedResult(t, o, earlier, true, 1) {
		t.Fatal("earlier result could not enter empty bounded queue")
	}
	if projectOrderedResult(t, o, later, true, 1) {
		t.Fatal("queue exceeded maximum")
	}
	first := claimOrdered(t, o, now, "node-2")
	if err := o.SaveAttempt(ctx, *first, true, nil, 204, "", now); err != nil {
		t.Fatal(err)
	}
	if sequence, err := o.NextSourceEvent(ctx, cursor); err != nil || sequence != 1 {
		t.Fatalf("later event missing: %d %v", sequence, err)
	}
	if !projectOrderedResult(t, o, later, true, 1) {
		t.Fatal("later result blocked after acknowledgement")
	}
	projectOrderedResult(t, o, later, true, 1)
	var projected int
	if err := o.db.QueryRow(`SELECT webhook_cursor FROM v2_state WHERE execution_id='execution'`).Scan(&projected); err != nil || projected != 2 {
		t.Fatal("source projection count/replay", projected, err)
	}
	claimOrdered(t, o, now, "node-1")
}

func TestSourceConcurrentAdmissionIsOrderedAndTerminalWaitsForProjection(t *testing.T) {
	o, now := orderedOutbox(t, "source", 2)
	later := addOrderedResult(t, o, now, 1, 0, 1)
	earlier := addOrderedResult(t, o, now, 2, 0, 0)
	if _, err := o.db.Exec(`UPDATE v2_state SET terminal=1 WHERE execution_id='execution'`); err != nil {
		t.Fatal(err)
	}
	projectOrderedResult(t, o, later, true, 10)
	claimOrdered(t, o, now, "")
	projectOrderedResult(t, o, earlier, true, 10)
	claimOrdered(t, o, now, "node-2")
	claimOrdered(t, o, now, "node-1")
	claimOrdered(t, o, now, "")
}

func TestOrderedRecoveryRetainsWindowAndExpiryUnblocksNext(t *testing.T) {
	ctx := context.Background()
	o, now := orderedOutbox(t, "completion", 1)
	first := addOrderedResult(t, o, now, 1, 0, 0)
	first.DeadlineAt = now.Add(10 * time.Second)
	projectOrderedResult(t, o, first, false, 10)
	projectOrderedResult(t, o, addOrderedResult(t, o, now, 2, 0, 1), false, 10)
	claimOrdered(t, o, now, "node-1")
	if err := o.Recover(ctx, "replacement", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := o.db.Exec(`UPDATE webhook_deliveries SET next_attempt_at=? WHERE id='node-1'`, now.Add(20*time.Second).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	claimOrdered(t, o, now.Add(2*time.Second), "")
	claimOrdered(t, o, now.Add(11*time.Second), "node-2")
	var status string
	if err := o.db.QueryRow(`SELECT status FROM webhook_deliveries WHERE id='node-1'`).Scan(&status); err != nil || status != "failed" {
		t.Fatal("expired recovered delivery", status, err)
	}
}

func TestSourceFailedPayloadReleasesOrderWithoutHTTP(t *testing.T) {
	o, now := orderedOutbox(t, "source", 1)
	first := addOrderedResult(t, o, now, 1, 0, 0)
	first.Failure = "WEBHOOK_PAYLOAD_TOO_LARGE"
	projectOrderedResult(t, o, first, true, 10)
	projectOrderedResult(t, o, addOrderedResult(t, o, now, 2, 0, 1), true, 10)
	if _, err := o.db.Exec(`UPDATE v2_state SET source_index=1 WHERE execution_id='execution'`); err != nil {
		t.Fatal(err)
	}
	claimOrdered(t, o, now, "node-2")
}
