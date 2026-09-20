package http_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/javleds/api-engine/internal/api/http/dto"
	definition "github.com/javleds/api-engine/internal/definition/domain"
)

func TestWebhookPolicyVersionBoundary(t *testing.T) {
	h, store, _ := newTestAPI(t)
	for _, test := range []webhookPolicyAPICase{
		{"", "completion", 1},
		{`,"ordering":"source","maxInFlight":4`, "source", 4},
	} {
		body := append(append([]byte{}, v2Request[:len(v2Request)-1]...), []byte(`,"webhook":{"url":"https://example.com/hook"`+test.fields+`}}`)...)
		response := request(t, h, "POST", "/v2/executions", body, "", "owner")
		if response.Code != 202 {
			t.Fatalf("%d %s", response.Code, response.Body)
		}
		var accepted dto.CreateExecutionResponse
		if err := json.Unmarshal(response.Body.Bytes(), &accepted); err != nil {
			t.Fatal(err)
		}
		snapshot, _, err := store.Load(context.Background(), accepted.ID)
		if err != nil {
			t.Fatal(err)
		}
		var compiled definition.CompiledDefinition
		if err := json.Unmarshal(snapshot.Definition, &compiled); err != nil {
			t.Fatal(err)
		}
		if compiled.Webhook.Ordering != test.ordering || compiled.Webhook.MaxInFlight != test.maxInFlight {
			t.Fatalf("persisted policy: %+v", compiled.Webhook)
		}
		if validation := request(t, h, "POST", "/playground/api/v2/validate", body, "", "owner"); validation.Code != 200 {
			t.Fatal(validation.Body)
		}
		if test.fields != "" {
			for _, path := range []string{"/v1/executions", "/playground/api/validate"} {
				if response := request(t, h, "POST", path, body, "", "owner"); response.Code != 422 {
					t.Fatalf("%s: %d %s", path, response.Code, response.Body)
				}
			}
		}
	}
}

func TestV2WebhookPolicyDefaultsPreserveIdempotency(t *testing.T) {
	h, _, _ := newTestAPI(t)
	var firstID string
	for _, fields := range []string{"", `,"ordering":"completion","maxInFlight":1`} {
		body := append(append([]byte{}, v2Request[:len(v2Request)-1]...), []byte(`,"webhook":{"url":"https://example.com/hook"`+fields+`}}`)...)
		response := request(t, h, "POST", "/v2/executions", body, "policy-defaults", "owner")
		if response.Code != 202 {
			t.Fatalf("%d %s", response.Code, response.Body)
		}
		var accepted dto.CreateExecutionResponse
		if err := json.Unmarshal(response.Body.Bytes(), &accepted); err != nil {
			t.Fatal(err)
		}
		if firstID != "" && firstID != string(accepted.ID) {
			t.Fatal("explicit defaults produced a different execution")
		}
		firstID = string(accepted.ID)
	}
}
