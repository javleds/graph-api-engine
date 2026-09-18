package application

import (
	"context"
	"encoding/json"
	"fmt"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	notification "github.com/javleds/api-engine/internal/notification/domain"
	"strings"
	"testing"
	"time"
)

func partialCompletion() execution.ExecutionCompletion {
	path, pointer := "items[0].detail", "/definition/calls/children/0"
	return execution.ExecutionCompletion{ID: "execution-id", Status: execution.CompletedWithErrors, FinishedAt: time.Unix(100, 0).UTC(), Data: []byte(`{"items":[{"detail":null}]}`), Report: execution.ExecutionErrorReport{Errors: []execution.NodeFailure{{Code: "UPSTREAM_NOT_FOUND", Message: "No disponible.", Scope: "node", NodePath: &path, DefinitionPath: &pointer, Stage: "fetch", Attempts: 1}}, ErrorSummary: execution.ErrorSummary{Total: 1, Returned: 1}, Roots: []execution.RootOutcome{{Name: "items", Status: execution.CompletedWithErrors, DataAvailable: true}}}}
}
func TestResultWebhookUsesDataAndStableEventIdentity(t *testing.T) {
	builder := WebhookPayloadBuilder{PublicBaseURL: "https://engine.test/prefix/"}
	completion := partialCompletion()
	data, err := builder.Build("event-id", definition.Webhook{Payload: "result"}, completion)
	if err != nil {
		t.Fatal(err)
	}
	var payload notification.WebhookPayload
	if err = json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Type != "execution.completed_with_errors" || payload.EventID != "event-id" || !payload.ResultIncluded || string(payload.Result) != string(completion.Data) || payload.Links.Self != "https://engine.test/prefix/v1/executions/execution-id" {
		t.Fatal(string(data))
	}
}
func TestPayloadDropsResultBeforeDetails(t *testing.T) {
	builder := WebhookPayloadBuilder{PublicBaseURL: "https://engine.test", MaxPayloadBytes: 1100}
	completion := partialCompletion()
	completion.Data = []byte(`{"large":"` + strings.Repeat("x", 2000) + `"}`)
	encoded, err := builder.Build("event", definition.Webhook{Payload: "result"}, completion)
	if err != nil {
		t.Fatal(err)
	}
	var payload notification.WebhookPayload
	_ = json.Unmarshal(encoded, &payload)
	if payload.ResultIncluded || payload.ResultOmittedReason != "size_limit" || !payload.ErrorsIncluded || len(payload.Errors) != 1 {
		t.Fatal(string(encoded))
	}
}
func TestPayloadDropsDetailsWithoutChangingPersistedReport(t *testing.T) {
	builder := WebhookPayloadBuilder{PublicBaseURL: "https://engine.test", MaxPayloadBytes: 950}
	completion := partialCompletion()
	completion.Report.Errors[0].Message = strings.Repeat("x", 2000)
	encoded, err := builder.Build("event", definition.Webhook{Payload: "status"}, completion)
	if err != nil {
		t.Fatal(err)
	}
	var payload notification.WebhookPayload
	_ = json.Unmarshal(encoded, &payload)
	if payload.ErrorsIncluded || payload.ErrorsOmittedReason != "size_limit" || payload.ErrorSummary.Returned != 0 || !payload.ErrorSummary.Truncated || len(completion.Report.Errors) != 1 {
		t.Fatal(string(encoded))
	}
}
func TestFailedWebhookNeverIncludesResult(t *testing.T) {
	builder := WebhookPayloadBuilder{PublicBaseURL: "https://engine.test"}
	completion := partialCompletion()
	completion.Status = execution.Failed
	completion.Error = &execution.ExecutionFailure{Code: "PROCESS_INTERRUPTED", Message: "El proceso se interrumpió."}
	encoded, err := builder.Build("event", definition.Webhook{Payload: "result"}, completion)
	if err != nil {
		t.Fatal(err)
	}
	var payload notification.WebhookPayload
	_ = json.Unmarshal(encoded, &payload)
	if payload.ResultIncluded || payload.Result != nil || payload.Type != "execution.failed" || payload.Error.Code != "PROCESS_INTERRUPTED" {
		t.Fatal(string(encoded))
	}
}
func TestPrepareCompletionFreezesPayloadAndEncryptedTargetInput(t *testing.T) {
	counter := 0
	builder := WebhookPayloadBuilder{PublicBaseURL: "https://engine.test", DeliveryMaxAge: time.Hour, NewID: func() (string, error) { counter++; return fmt.Sprintf("id-%d", counter), nil }}
	target := definition.Webhook{URL: "https://receiver.test/callback", Method: "POST", Auth: definition.Authentication{Type: "hmac", Secret: "secret"}, Payload: "result"}
	raw, _ := json.Marshal(definition.CompiledDefinition{Version: 1, Webhook: &target})
	completion := partialCompletion()
	if err := builder.PrepareCompletion(context.Background(), execution.ExecutionSnapshot{Definition: raw}, &completion); err != nil {
		t.Fatal(err)
	}
	if completion.Delivery.EventID != "id-1" || completion.Delivery.ID != "id-2" || !completion.Delivery.DeadlineAt.Equal(completion.FinishedAt.Add(time.Hour)) {
		t.Fatal(completion.Delivery)
	}
	var persistedTarget definition.Webhook
	_ = json.Unmarshal(completion.Delivery.Target, &persistedTarget)
	if persistedTarget.Auth.Secret != "secret" {
		t.Fatal("target must reach transaction protector intact")
	}
}
func TestBuilderRejectsImpossibleMinimumAtStartup(t *testing.T) {
	builder := WebhookPayloadBuilder{PublicBaseURL: "https://engine.test", MaxPayloadBytes: 100}
	if builder.Validate() == nil {
		t.Fatal("undersized envelope accepted")
	}
	builder.MaxPayloadBytes = 262144
	if err := builder.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareWithoutWebhookDoesNotReserveDelivery(t *testing.T) {
	raw, _ := json.Marshal(definition.CompiledDefinition{Version: 1})
	completion := partialCompletion()
	builder := WebhookPayloadBuilder{}
	if err := builder.PrepareCompletion(context.Background(), execution.ExecutionSnapshot{Definition: raw}, &completion); err != nil || completion.Delivery != nil {
		t.Fatal(err, completion.Delivery)
	}
}
func TestCanceledPreparationAndInvalidSnapshotCannotCreateDelivery(t *testing.T) {
	builder := WebhookPayloadBuilder{}
	completion := partialCompletion()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := builder.PrepareCompletion(ctx, execution.ExecutionSnapshot{}, &completion); err == nil {
		t.Fatal("canceled preparation accepted")
	}
	if err := builder.PrepareCompletion(context.Background(), execution.ExecutionSnapshot{Definition: []byte(`broken`)}, &completion); err == nil {
		t.Fatal("corrupted snapshot accepted")
	}
}
func TestOversizedMinimalPayloadDoesNotTruncateJSON(t *testing.T) {
	builder := WebhookPayloadBuilder{MaxPayloadBytes: 20}
	if _, err := builder.Build("event", definition.Webhook{Payload: "result"}, partialCompletion()); err == nil {
		t.Fatal("impossible payload accepted")
	}
}
func TestInvalidResultIsNotAnnouncedAsIncluded(t *testing.T) {
	builder := WebhookPayloadBuilder{}
	completion := partialCompletion()
	completion.Data = []byte(`broken`)
	if _, err := builder.Build("event", definition.Webhook{Payload: "result"}, completion); err == nil {
		t.Fatal("invalid result published")
	}
}
func TestDefaultPreparationCreatesDistinctStableIdentifiers(t *testing.T) {
	raw, _ := json.Marshal(definition.CompiledDefinition{Version: 1, Webhook: &definition.Webhook{URL: "https://receiver.test", Method: "POST"}})
	builder := WebhookPayloadBuilder{}
	completion := partialCompletion()
	if err := builder.PrepareCompletion(context.Background(), execution.ExecutionSnapshot{Definition: raw}, &completion); err != nil {
		t.Fatal(err)
	}
	if completion.Delivery.ID == completion.Delivery.EventID || completion.Delivery.ID == "" || !completion.Delivery.DeadlineAt.Equal(completion.FinishedAt.Add(24*time.Hour)) {
		t.Fatal(completion.Delivery)
	}
}
