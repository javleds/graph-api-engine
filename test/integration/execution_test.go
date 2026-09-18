package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// I04/I05/I07/I09 and E01/E34/E40: HTTP, durable partial results and outbox restart.
func TestPartialExecutionCacheIdempotencyAndWebhookRestart(t *testing.T) {
	var lists atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/items":
			lists.Add(1)
			io.WriteString(w, `[{"id":9007199254740993},{"id":2}]`)
		case "/detail/9007199254740993":
			io.WriteString(w, `{"ok":true}`)
		default:
			w.WriteHeader(404)
			io.WriteString(w, `{"private":"must not leak"}`)
		}
	}))
	defer upstream.Close()
	delivered := make(chan capturedWebhook, 4)
	var attempts atomic.Int32
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		delivered <- capturedWebhook{Header: r.Header.Clone(), Body: body}
		if attempts.Add(1) == 1 {
			w.WriteHeader(503)
		} else {
			w.WriteHeader(204)
		}
	}))
	defer receiver.Close()
	cfg := fixtureConfig(t, upstream.URL, receiver.URL)
	fixture := openFixture(t, cfg)
	webhook := map[string]any{"url": receiver.URL, "auth": map[string]any{"type": "hmac", "secret": "integration-shared-secret"}, "payload": "result"}
	body := definitionBody(upstream.URL, webhook)
	id := fixture.create(t, body, "incremental-test")
	if replay := fixture.create(t, body, "incremental-test"); replay != id {
		t.Fatal("idempotent replay created another execution")
	}
	terminal := fixture.terminal(t, id)
	if terminal["status"] != "completed_with_errors" {
		t.Fatalf("expected partial: %#v", terminal)
	}
	status, result := fixture.request(t, "GET", "/v1/executions/"+id+"/result", fixture.token, "", nil)
	if status != 200 {
		t.Fatalf("result %d: %s", status, result)
	}
	validateResultSchema(t, result)
	if !bytes.Contains(result, []byte(`9007199254740993`)) || bytes.Contains(result, []byte("must not leak")) {
		t.Fatalf("precision or redaction failed: %s", result)
	}
	if status, _ := fixture.request(t, "GET", "/v1/executions/"+id, fixture.otherToken, "", nil); status != 404 {
		t.Fatalf("owner isolation status %d", status)
	}
	if status, _ := fixture.request(t, "GET", "/v1/executions/"+id, "", "", nil); status != 401 {
		t.Fatalf("missing auth status %d", status)
	}
	changed := bytes.Replace(body, []byte(`"name":"items"`), []byte(`"name":"other"`), 1)
	if status, _ := fixture.request(t, "POST", "/v1/executions", fixture.token, "incremental-test", changed); status != 409 {
		t.Fatalf("idempotency conflict %d", status)
	}
	second := fixture.create(t, definitionBody(upstream.URL, nil), "")
	fixture.terminal(t, second)
	if lists.Load() != 1 {
		t.Fatalf("expected cached root, got %d upstream list calls", lists.Load())
	}
	if err := fixture.app.Notifications.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	first := <-delivered
	fixture.close(t)
	fixture = openFixture(t, cfg)
	deadline := time.Now().Add(2 * time.Second)
	for attempts.Load() < 2 && time.Now().Before(deadline) {
		if err := fixture.app.Notifications.DispatchOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	if attempts.Load() != 2 {
		t.Fatal("pending webhook was not recovered")
	}
	secondDelivery := <-delivered
	if !bytes.Equal(first.Body, secondDelivery.Body) || first.Header.Get("X-Engine-Event-Id") != secondDelivery.Header.Get("X-Engine-Event-Id") {
		t.Fatal("webhook identity or immutable body changed after restart")
	}
	assertSignature(t, secondDelivery, "integration-shared-secret")
	status, persisted := fixture.request(t, "GET", "/v1/executions/"+id+"/result", fixture.token, "", nil)
	if status != 200 || !bytes.Equal(result, persisted) {
		t.Fatal("durable result changed on restart")
	}
	if lists.Load() != 1 {
		t.Fatal("restart reexecuted the tree")
	}
}
func assertSignature(t *testing.T, capture capturedWebhook, secret string) {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(capture.Header.Get("X-Engine-Event-Id") + "." + capture.Header.Get("X-Engine-Timestamp") + "."))
	mac.Write(capture.Body)
	if capture.Header.Get("X-Engine-Signature") != "v1="+hex.EncodeToString(mac.Sum(nil)) {
		t.Fatal("invalid HMAC signature")
	}
	var payload map[string]any
	if err := json.Unmarshal(capture.Body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["status"] != "completed_with_errors" {
		t.Fatalf("wrong webhook status: %s", capture.Body)
	}
}
func validateResultSchema(t *testing.T, result []byte) {
	t.Helper()
	source, err := os.ReadFile("../../contracts/execution_result_v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("urn:result", document); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("urn:result")
	if err != nil {
		t.Fatal(err)
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(result))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(value); err != nil {
		t.Fatalf("terminal schema: %v; body: %s", err, result)
	}
}
