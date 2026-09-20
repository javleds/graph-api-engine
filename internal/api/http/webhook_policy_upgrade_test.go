package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	api "github.com/javleds/api-engine/internal/api/http"
	"github.com/javleds/api-engine/internal/api/http/dto"
	"github.com/javleds/api-engine/internal/api/http/mapper"
	definition "github.com/javleds/api-engine/internal/definition/application"
	"github.com/javleds/api-engine/internal/definition/infrastructure/schema"
	"github.com/javleds/api-engine/internal/execution/application"
	"github.com/javleds/api-engine/internal/platform/clock"
	"github.com/javleds/api-engine/internal/platform/identifier"
)

func TestWebhookPolicyUpgradeReplaysHistoricalRequest(t *testing.T) {
	_, store, dispatcher := newTestAPI(t)
	create := &application.CreateExecution{Store: store, Dispatcher: dispatcher, IDs: identifier.UUIDGenerator{}, Clock: clock.RealClock{}, IdentityKey: []byte("stable-identity-key"), MetadataRetention: time.Hour}
	server := &api.Server{Create: create, Reader: store, V2Store: store, Auth: testAuthenticator{}, Limits: definition.DefaultValidationLimits(), MaxRequestBytes: 262144, MaxJSONDepth: 64}
	handler := server.Handler()
	body := append(append([]byte{}, v2Request[:len(v2Request)-1]...), []byte(`,"webhook":{"url":"https://example.com/hook"}}`)...)
	// Historical parsing did not add a delivery policy to the compiled snapshot.
	parsed, err := schema.Parse(body, 64)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := mapper.Compile(parsed, definition.DefaultValidationLimits())
	if err != nil {
		t.Fatal(err)
	}
	legacySnapshot, err := json.Marshal(compiled)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := create.Execute(context.Background(), application.CreateExecutionCommand{APIVersion: 2, OwnerID: "owner", IdempotencyKey: "before-upgrade", Definition: legacySnapshot, WebhookEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, fields := range []string{"", `,"ordering":"completion","maxInFlight":1`} {
		requestBody := append(append([]byte{}, body[:len(body)-2]...), []byte(fields+`}}`)...)
		response := request(t, handler, "POST", "/v2/executions", requestBody, "before-upgrade", "owner")
		if response.Code != 202 {
			t.Fatalf("replay: %d %s", response.Code, response.Body)
		}
		var replay dto.CreateExecutionResponse
		if err := json.Unmarshal(response.Body.Bytes(), &replay); err != nil {
			t.Fatal(err)
		}
		if replay.ID != legacy.Execution.ID {
			t.Fatal("historical request was not replayed")
		}
	}
	for _, fields := range []string{`,"ordering":"source"`, `,"maxInFlight":2`} {
		requestBody := append(append([]byte{}, body[:len(body)-2]...), []byte(fields+`}}`)...)
		response := request(t, handler, "POST", "/v2/executions", requestBody, "before-upgrade", "owner")
		if response.Code != 409 {
			t.Fatalf("changed policy: %d %s", response.Code, response.Body)
		}
	}
	persisted, _, err := store.Load(context.Background(), legacy.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(persisted.Definition, legacySnapshot) {
		t.Fatal("replay rewrote historical snapshot")
	}
}
