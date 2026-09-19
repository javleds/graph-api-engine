package application

import (
	"context"
	"encoding/json"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	"strings"
	"testing"
	"time"
)

func TestV2WebhookLinksAndClosure(t *testing.T) {
	builder := WebhookPayloadBuilder{PublicBaseURL: "https://engine.example", NewID: func() (string, error) { return "id", nil }}
	for _, owner := range []string{"owner", "system:playground"} {
		snapshot := execution.ExecutionSnapshot{APIVersion: 2, OwnerID: owner, Definition: []byte(`{"version":1,"webhook":{"url":"https://callback.example","payload":"status"}}`)}
		c := execution.ExecutionCompletion{APIVersion: 2, ID: "job", Status: execution.Failed, Error: &execution.ExecutionFailure{Code: "EXECUTION_TIMEOUT", Message: "expired"}, Report: execution.EmptyReport(), LastSequence: "123", FinishedAt: time.Now(), ResultExpiresAt: time.Now().Add(time.Hour)}
		if err := builder.PrepareCompletion(context.Background(), snapshot, &c); err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err := json.Unmarshal(c.Delivery.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if payload["lastSequence"] != "123" || payload["resultIncluded"] != false || payload["resultExpiresAt"] == nil {
			t.Fatalf("closure %s", c.Delivery.Payload)
		}
		links := payload["links"].(map[string]any)
		prefix := "https://engine.example/v2/executions/job"
		if owner == "system:playground" {
			prefix = "https://engine.example/playground/api/v2/executions/job"
		}
		if links["chunks"] != prefix+"/chunks" || links["export"] != prefix+"/export" || links["result"] != nil || !strings.HasPrefix(links["self"].(string), prefix) {
			t.Fatalf("links %v", links)
		}
	}
}
