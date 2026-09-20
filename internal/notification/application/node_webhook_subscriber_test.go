package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	execution "github.com/javleds/api-engine/internal/execution/domain"
)

func nodeSubscriberFixture(now time.Time) (*NodeWebhookSubscriber, *nodeResults, *nodeOutbox) {
	results := &nodeResults{event: execution.V2Result{EventID: "job:node", ExecutionID: "job", NodeID: "node", Sequence: "1", Type: "node.finished", Status: "completed", OccurredAt: now}, data: `{"number":9007199254740993}`}
	outbox := &nodeOutbox{}
	s := &NodeWebhookSubscriber{Outbox: outbox, Store: nodeSnapshots{}, Results: results, Builder: &WebhookPayloadBuilder{PublicBaseURL: "https://engine.example", DeliveryMaxAge: time.Hour}, Clock: fixedClock{now}}
	return s, results, outbox
}
func TestNodeWebhookPayloadExactLimitAndNumberPrecision(t *testing.T) {
	now := time.Now()
	s, results, _ := nodeSubscriberFixture(now)
	payload, err := s.buildNodePayload(context.Background(), "owner", results.event, now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"result":{"number":9007199254740993}`) {
		t.Fatalf("changed number: %s", payload)
	}
	s.Builder.MaxPayloadBytes = len(payload)
	exact, err := s.buildNodePayload(context.Background(), "owner", results.event, now)
	if err != nil || string(exact) != string(payload) {
		t.Fatalf("exact limit: %s %v", exact, err)
	}
	s.Builder.MaxPayloadBytes--
	if _, err = s.buildNodePayload(context.Background(), "owner", results.event, now); !errors.Is(err, errNodePayloadTooLarge) {
		t.Fatalf("oversized accepted: %v", err)
	}
	results.data = `"` + strings.Repeat("x", 10000) + `"`
	if _, err = s.buildNodePayload(context.Background(), "owner", results.event, now); !errors.Is(err, errNodePayloadTooLarge) {
		t.Fatalf("WriteString bypassed bound: %v", err)
	}
}
func TestNodeProjectionOversizeExpiryAndCapacity(t *testing.T) {
	for _, kind := range []string{"oversize", "expired", "full", "success"} {
		t.Run(kind, func(t *testing.T) {
			now := time.Now()
			s, results, outbox := nodeSubscriberFixture(now)
			switch kind {
			case "oversize":
				s.Builder.MaxPayloadBytes = 100
			case "expired":
				results.event.OccurredAt = now.Add(-2 * time.Hour)
			case "full":
				outbox.full = true
			}
			if err := s.ProjectOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			if kind == "full" {
				if outbox.after != 0 || len(outbox.saved) != 0 {
					t.Fatal("full outbox advanced cursor")
				}
				return
			}
			if len(outbox.saved) != 1 || outbox.after != 1 {
				t.Fatalf("projection: %+v", outbox)
			}
			d := outbox.saved[0]
			if kind == "oversize" && (d.Failure != "WEBHOOK_PAYLOAD_TOO_LARGE" || len(d.Payload) != 0) {
				t.Fatalf("oversize: %+v", d)
			}
			if kind == "expired" && (d.Failure != "DELIVERY_DEADLINE_EXCEEDED" || results.writes != 0) {
				t.Fatalf("expiry: %+v", d)
			}
			if kind == "success" && (d.Failure != "" || len(d.Payload) == 0) {
				t.Fatalf("success: %+v", d)
			}
			if err := s.ProjectOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			if len(outbox.saved) != 1 {
				t.Fatal("replayed event")
			}
		})
	}
}
