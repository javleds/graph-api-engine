package http_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	api "github.com/javleds/api-engine/internal/api/http"
	definitionapp "github.com/javleds/api-engine/internal/definition/application"
	"github.com/javleds/api-engine/internal/execution/application"
	"github.com/javleds/api-engine/internal/execution/domain"
	executionsql "github.com/javleds/api-engine/internal/execution/infrastructure/sqlite"
	"github.com/javleds/api-engine/internal/platform/clock"
	"github.com/javleds/api-engine/internal/platform/identifier"
	platformsql "github.com/javleds/api-engine/internal/platform/sqlite"
	security "github.com/javleds/api-engine/internal/security/infrastructure"
)

func newTestAPI(t *testing.T) (http.Handler, *executionsql.ExecutionStore, *testDispatcher) {
	t.Helper()
	db, err := platformsql.Open(context.Background(), filepath.Join(t.TempDir(), "engine.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := platformsql.Migrate(context.Background(), db, "engine"); err != nil {
		t.Fatal(err)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	protector, err := security.NewAESGCMProtector("test", map[string][]byte{"test": key})
	if err != nil {
		t.Fatal(err)
	}
	store := executionsql.NewExecutionStore(db, protector)
	dispatcher := &testDispatcher{}
	create := &application.CreateExecution{Store: store, Dispatcher: dispatcher, Finalizer: executionsql.NewExecutionFinalizer(store), IDs: identifier.UUIDGenerator{}, Clock: clock.RealClock{}, IdentityKey: key, MetadataRetention: 30 * 24 * time.Hour}
	cancel := &application.CancelExecution{Store: store, Reader: store, Finalizer: &application.FinalizeExecution{Store: store, Writer: executionsql.NewExecutionFinalizer(store)}, Canceller: dispatcher, Clock: clock.RealClock{}}
	server := &api.Server{Cancel: cancel, PlaygroundAuth: security.PlaygroundAuthenticator{}, Create: create, Reader: store, V2Store: store, Results: store, Auth: testAuthenticator{}, Clock: clock.RealClock{}, Limits: definitionapp.DefaultValidationLimits(), MaxRequestBytes: 262144, MaxJSONDepth: 64}
	return server.Handler(), store, dispatcher
}
func request(t *testing.T, h http.Handler, method, path string, body []byte, key, owner string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+owner)
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestAcceptanceReplayAndOwnerIsolation(t *testing.T) {
	h, _, dispatcher := newTestAPI(t)
	body, err := os.ReadFile("../../../contracts/examples/pokemon_full.json")
	if err != nil {
		t.Fatal(err)
	}
	first := request(t, h, "POST", "/v1/executions", body, "same", "owner")
	if first.Code != 202 {
		t.Fatalf("%d %s", first.Code, first.Body)
	}
	var accepted map[string]any
	if err := json.Unmarshal(first.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	id := accepted["id"].(string)
	dispatcher.busy = true
	replay := request(t, h, "POST", "/v1/executions", body, "same", "owner")
	if replay.Code != 202 || dispatcher.commits != 1 {
		t.Fatalf("replay: %d dispatches=%d %s", replay.Code, dispatcher.commits, replay.Body)
	}
	if got := request(t, h, "POST", "/v1/executions", body, "new", "owner"); got.Code != 503 {
		t.Fatal(got.Code)
	}
	if got := request(t, h, "GET", "/v1/executions/"+id, nil, "", "other"); got.Code != 404 {
		t.Fatal(got.Code)
	}
	if got := request(t, h, "GET", "/v1/executions/"+id, nil, "", "owner"); got.Code != 200 {
		t.Fatal(got.Code)
	}
	if got := request(t, h, "GET", "/v1/executions/"+id+"/result", nil, "", "owner"); got.Code != 409 {
		t.Fatal(got.Code)
	}
}
func TestStrictJSONAndAuthentication(t *testing.T) {
	h, _, _ := newTestAPI(t)
	if got := request(t, h, "POST", "/v1/executions", []byte(`{"definition":1,"definition":2}`), "", "owner"); got.Code != 400 {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	if got := request(t, h, "POST", "/v1/executions", []byte(`{}`), "", ""); got.Code != 401 {
		t.Fatal(got.Code)
	}
	if got := request(t, h, "POST", "/v1/executions", []byte(`{"unknown":true}`), "", "owner"); got.Code != 422 {
		t.Fatal(got.Code)
	}
}
func TestPartialResultEnvelope(t *testing.T) {
	h, store, _ := newTestAPI(t)
	body := []byte(`{"definition":{"version":1,"api":{"baseUrl":"https://example.com","limits":{"requests":100,"per":"1m"}},"calls":[{"name":"items","type":"collection","request":{"method":"GET","path":"/items"}}]}}`)
	accepted := request(t, h, "POST", "/v1/executions", body, "", "owner")
	if accepted.Code != 202 {
		t.Fatalf("%d %s", accepted.Code, accepted.Body)
	}
	var result map[string]any
	json.Unmarshal(accepted.Body.Bytes(), &result)
	id := domain.ExecutionID(result["id"].(string))
	now := time.Now()
	report := domain.EmptyReport()
	report.Roots = []domain.RootOutcome{{Name: "items", Status: "completed", DataAvailable: true}}
	if ok, err := store.Claim(context.Background(), id, "test-worker", 0, now); err != nil || !ok {
		t.Fatalf("claim: %v", err)
	}
	err := executionsql.NewExecutionFinalizer(store).Finish(context.Background(), domain.ExecutionCompletion{ID: id, ExpectedVersion: 1, Status: domain.Completed, Report: report, Data: []byte(`{"items":[]}`), FinishedAt: now, ResultExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	got := request(t, h, "GET", "/v1/executions/"+string(id)+"/result", nil, "", "owner")
	if got.Code != 200 {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	if !bytes.Contains(got.Body.Bytes(), []byte(`"errors":[]`)) || !bytes.Contains(got.Body.Bytes(), []byte(`"result":{"items":[]}`)) {
		t.Fatal(got.Body)
	}
}
