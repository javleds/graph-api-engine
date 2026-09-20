package application

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	definition "github.com/javleds/api-engine/internal/definition/domain"
	executionports "github.com/javleds/api-engine/internal/execution/application/ports"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/notification/application/ports"
	"github.com/javleds/api-engine/internal/notification/domain"
	"github.com/javleds/api-engine/internal/platform/clock"
)

// NodeWebhookSubscriber projects durable events into independently retryable deliveries.
type NodeWebhookSubscriber struct {
	Outbox         ports.NodeEventOutbox
	Store          executionports.ExecutionStore
	Results        executionports.V2ResultStore
	Builder        *WebhookPayloadBuilder
	MaxPending     int
	MaxStoredBytes int64
	PollInterval   time.Duration
	Clock          clock.Clock
	OnError        func(error)
	afterExecution string
}

func (s *NodeWebhookSubscriber) Run(ctx context.Context) {
	interval := s.PollInterval
	if interval <= 0 {
		interval = time.Second
	}
	for ctx.Err() == nil {
		if err := s.ProjectOnce(ctx); err != nil && s.OnError != nil {
			s.OnError(err)
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *NodeWebhookSubscriber) ProjectOnce(ctx context.Context) error {
	cursors, err := s.Outbox.PendingNodeExecutions(ctx, s.afterExecution, 32)
	if err != nil {
		return err
	}
	if len(cursors) == 0 {
		s.afterExecution = ""
		return nil
	}
	source := s.Clock
	if source == nil {
		source = clock.RealClock{}
	}
	var failures []error
	for _, cursor := range cursors {
		for count := 0; count < 16; count++ {
			advanced, err := s.projectExecution(ctx, cursor, source.Now())
			if err != nil {
				failures = append(failures, err)
				break
			}
			if !advanced {
				break
			}
			cursor.After++
		}
		s.afterExecution = string(cursor.ExecutionID)
	}
	return errors.Join(failures...)
}

func (s *NodeWebhookSubscriber) projectExecution(ctx context.Context, cursor domain.NodeEventCursor, now time.Time) (bool, error) {
	// Expired events still need a failed delivery record; never reconstruct their data.
	page, err := s.Results.ReadV2Results(ctx, cursor.OwnerID, cursor.ExecutionID, cursor.After, 0, 1, time.Time{})
	if err != nil {
		return false, err
	}
	if len(page.Results) == 0 {
		return false, nil
	}
	event := page.Results[0]
	snapshot, _, err := s.Store.Load(ctx, cursor.ExecutionID)
	if err != nil {
		return false, err
	}
	var compiled definition.CompiledDefinition
	if err = json.Unmarshal(snapshot.Definition, &compiled); err != nil {
		return false, err
	}
	if compiled.Webhook == nil {
		return false, errors.New("webhook event has no persisted target")
	}
	target, err := json.Marshal(compiled.Webhook)
	if err != nil {
		return false, err
	}
	sequence, err := strconv.ParseInt(event.Sequence, 10, 64)
	if err != nil {
		return false, err
	}
	age := s.Builder.DeliveryMaxAge
	if age <= 0 {
		age = 24 * time.Hour
	}
	delivery := domain.PreparedNodeDelivery{WebhookDelivery: domain.WebhookDelivery{
		ID: event.EventID, EventID: event.EventID, ExecutionID: event.ExecutionID, OwnerID: cursor.OwnerID,
		EventType: "node.finished", Target: target, DeadlineAt: event.OccurredAt.Add(age), NextAttemptAt: now,
	}, Sequence: sequence, OccurredAt: event.OccurredAt}
	if !now.Before(delivery.DeadlineAt) {
		delivery.Failure = "DELIVERY_DEADLINE_EXCEEDED"
	} else {
		delivery.Payload, err = s.buildNodePayload(ctx, cursor.OwnerID, event, now)
		if errors.Is(err, errNodePayloadTooLarge) {
			delivery.Failure = "WEBHOOK_PAYLOAD_TOO_LARGE"
		} else if err != nil {
			return false, err
		}
	}
	return s.Outbox.SaveNodeDelivery(ctx, cursor, delivery, s.MaxPending, s.MaxStoredBytes)
}

var errNodePayloadTooLarge = errors.New("node webhook payload exceeds limit")

func (s *NodeWebhookSubscriber) buildNodePayload(ctx context.Context, owner string, event execution.V2Result, now time.Time) ([]byte, error) {
	buffer := &boundedPayload{maximum: s.Builder.maximum()}
	if err := s.Results.WriteV2Result(ctx, owner, event.ExecutionID, event.NodeID, now, buffer); err != nil {
		return nil, err
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(buffer.buffer.Bytes(), &envelope); err != nil {
		return nil, err
	}
	data, ok := envelope["data"]
	if !ok {
		return nil, errors.New("node result envelope has no data")
	}
	delete(envelope, "data")
	envelope["result"] = data
	prefix := "/v2/executions/"
	if owner == "system:playground" {
		prefix = "/playground/api/v2/executions/"
	}
	self := strings.TrimRight(s.Builder.PublicBaseURL, "/") + prefix + string(event.ExecutionID)
	links, _ := json.Marshal(map[string]string{"self": self, "result": self + "/results/" + event.NodeID})
	envelope["links"] = links
	payload, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	if len(payload) > s.Builder.maximum() {
		return nil, errNodePayloadTooLarge
	}
	return payload, nil
}
