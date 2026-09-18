package application

import (
	"context"
	"encoding/json"
	"errors"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/notification/domain"
	"github.com/javleds/api-engine/internal/platform/identifier"
	"math"
	"strings"
	"time"
)

// WebhookPayloadBuilder freezes event identity and body before the terminal transaction.
type WebhookPayloadBuilder struct {
	PublicBaseURL        string
	MaxPayloadBytes      int
	DeliveryMaxAge       time.Duration
	NewID                func() (string, error)
	MaxRootCalls         int
	MaxErrorMessageBytes int
}

func (b *WebhookPayloadBuilder) PrepareCompletion(ctx context.Context, snapshot execution.ExecutionSnapshot, completion *execution.ExecutionCompletion) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var compiled definition.CompiledDefinition
	if err := json.Unmarshal(snapshot.Definition, &compiled); err != nil {
		return errors.New("invalid persisted webhook definition")
	}
	if compiled.Webhook == nil {
		return nil
	}
	newID := b.NewID
	if newID == nil {
		newID = identifier.UUIDGenerator{}.NewID
	}
	eventID, err := newID()
	if err != nil {
		return err
	}
	deliveryID, err := newID()
	if err != nil {
		return err
	}
	payload, err := b.Build(eventID, *compiled.Webhook, *completion)
	if err != nil {
		return err
	}
	target, err := json.Marshal(compiled.Webhook)
	if err != nil {
		return err
	}
	age := b.DeliveryMaxAge
	if age <= 0 {
		age = 24 * time.Hour
	}
	completion.Delivery = &execution.WebhookRecord{ID: deliveryID, EventID: eventID, Target: target, Payload: payload, DeadlineAt: completion.FinishedAt.Add(age)}
	return nil
}
func (b *WebhookPayloadBuilder) Build(eventID string, target definition.Webhook, completion execution.ExecutionCompletion) ([]byte, error) {
	payload := b.basePayload(eventID, completion)
	if target.Payload == "result" && completion.Status != execution.Failed {
		if !json.Valid(completion.Data) {
			return nil, errors.New("webhook result is not valid JSON")
		}
		payload.ResultIncluded = true
		payload.Result = append(json.RawMessage(nil), completion.Data...)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if len(encoded) <= b.maximum() {
		return encoded, nil
	}
	if payload.ResultIncluded {
		payload.Result = nil
		payload.ResultIncluded = false
		payload.ResultOmittedReason = "size_limit"
		encoded, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		if len(encoded) <= b.maximum() {
			return encoded, nil
		}
	}
	payload.Errors = []execution.NodeFailure{}
	payload.ErrorsIncluded = false
	payload.ErrorsOmittedReason = "size_limit"
	payload.ErrorSummary.Returned = 0
	payload.ErrorSummary.Truncated = payload.ErrorSummary.Total > 0
	encoded, err = json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if len(encoded) > b.maximum() {
		return nil, errors.New("webhook minimum envelope exceeds configured limit")
	}
	return encoded, nil
}
func (b *WebhookPayloadBuilder) basePayload(eventID string, completion execution.ExecutionCompletion) domain.WebhookPayload {
	self := strings.TrimRight(b.PublicBaseURL, "/") + "/v1/executions/" + string(completion.ID)
	details := completion.Report.Errors
	if details == nil {
		details = []execution.NodeFailure{}
	}
	roots := completion.Report.Roots
	if roots == nil {
		roots = []execution.RootOutcome{}
	}
	return domain.WebhookPayload{EventID: eventID, Type: "execution." + string(completion.Status), ID: completion.ID, Status: completion.Status, OccurredAt: completion.FinishedAt, ErrorsIncluded: true, Errors: details, ErrorSummary: completion.Report.ErrorSummary, Roots: roots, Error: completion.Error, Links: domain.WebhookLinks{Self: self, Result: self + "/result"}}
}
func (b *WebhookPayloadBuilder) maximum() int {
	if b.MaxPayloadBytes <= 0 {
		return 262144
	}
	return b.MaxPayloadBytes
}
func (b *WebhookPayloadBuilder) Validate() error {
	roots := b.MaxRootCalls
	if roots <= 0 {
		roots = 16
	}
	messageBytes := b.MaxErrorMessageBytes
	if messageBytes <= 0 {
		messageBytes = 512
	}
	completion := execution.ExecutionCompletion{ID: execution.ExecutionID(strings.Repeat("i", 36)), Status: execution.CompletedWithErrors, FinishedAt: time.Now(), Error: &execution.ExecutionFailure{Code: "DELIVERY_DEADLINE_EXCEEDED", Message: strings.Repeat("x", messageBytes)}}
	completion.Report.ErrorSummary = execution.ErrorSummary{Total: math.MaxUint64, Truncated: true}
	for index := 0; index < roots; index++ {
		completion.Report.Roots = append(completion.Report.Roots, execution.RootOutcome{Name: strings.Repeat("x", 64), Status: execution.CompletedWithErrors, DataAvailable: true})
	}
	payload := b.basePayload(strings.Repeat("e", 36), completion)
	payload.ResultOmittedReason = "size_limit"
	payload.ErrorsOmittedReason = "size_limit"
	payload.ErrorsIncluded = false
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if len(encoded) > b.maximum() {
		return errors.New("maxPayloadBytes cannot contain the minimum webhook envelope")
	}
	return nil
}
