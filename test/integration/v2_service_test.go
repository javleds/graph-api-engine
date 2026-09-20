package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/javleds/api-engine/internal/api/http/dto"
	"github.com/javleds/api-engine/internal/bootstrap"
	"github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/platform/config"
)

func startV2Service(t *testing.T, cfg *config.Config) (*bootstrap.Application, string, func()) {
	t.Helper()
	app, err := bootstrap.Build(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		app.Close()
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Serve(ctx, listener) }()
	stopped := false
	stop := func() {
		t.Helper()
		if stopped {
			return
		}
		stopped = true
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("v2 service failed to shut down")
		}
		app.Close()
	}
	t.Cleanup(stop)
	return app, "http://" + listener.Addr().String(), stop
}
func v2ServiceRequest(t *testing.T, base, method, path, token string, body []byte) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, base+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, raw
}

func TestV2ServiceResumesCommittedParentAndDeliversTerminalWebhook(t *testing.T) {
	var parents, children atomic.Int32
	started := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/items" {
			parents.Add(1)
			io.WriteString(w, `[{"id":7}]`)
			return
		}
		if children.Add(1) == 1 {
			close(started)
			<-r.Context().Done()
			return
		}
		io.WriteString(w, `{"number":9007199254740993}`)
	}))
	t.Cleanup(upstream.Close)
	deliveries := make(chan capturedWebhook, 4)
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		deliveries <- capturedWebhook{Header: r.Header.Clone(), Body: raw}
		w.WriteHeader(204)
	}))
	t.Cleanup(webhook.Close)
	cfg := fixtureConfig(t, upstream.URL, webhook.URL)
	cfg.Fetch.AttemptTimeout = config.Duration(2 * time.Second)
	cfg.Shutdown.GracePeriod = config.Duration(50 * time.Millisecond)
	app, base, stop := startV2Service(t, cfg)
	var input map[string]any
	json.Unmarshal(definitionBody(upstream.URL, map[string]any{"url": webhook.URL, "payload": "status"}), &input)
	input["definition"].(map[string]any)["api"].(map[string]any)["cache"] = false
	body, _ := json.Marshal(input)
	token := "owner-one-a-long-random-looking-integration-token"
	code, raw := v2ServiceRequest(t, base, "POST", "/v2/executions", token, body)
	if code != 202 {
		t.Fatalf("accept %d %s", code, raw)
	}
	var accepted dto.CreateExecutionResponse
	if err := json.Unmarshal(raw, &accepted); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		_, status := v2ServiceRequest(t, base, "GET", "/v2/executions/"+string(accepted.ID), token, nil)
		t.Fatalf("child never started parent=%d child=%d status=%s", parents.Load(), children.Load(), status)
	}
	path := "/v2/executions/" + string(accepted.ID)
	code, raw = v2ServiceRequest(t, base, "GET", path+"/chunks", token, nil)
	var before domain.V2ChunkPage
	json.Unmarshal(raw, &before)
	if code != 200 || before.Terminal || len(before.Chunks) == 0 {
		t.Fatalf("no progressive committed result %d %s", code, raw)
	}
	state, err := app.Store.ReadV2State(context.Background(), accepted.ID)
	if err != nil {
		t.Fatal(err)
	}
	stop()
	_, base, _ = startV2Service(t, cfg)
	var terminal dto.ExecutionResponse
	deadline := time.After(10 * time.Second)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		code, raw = v2ServiceRequest(t, base, "GET", path, token, nil)
		if code != 200 {
			t.Fatalf("status %d %s", code, raw)
		}
		if err := json.Unmarshal(raw, &terminal); err != nil {
			t.Fatal(err)
		}
		if terminal.Status.Terminal() {
			break
		}
		select {
		case <-ticker.C:
		case <-deadline:
			t.Fatal("resumed job did not finish")
		}
	}
	if terminal.Status != domain.Completed || terminal.ID != accepted.ID || terminal.V2.Recoveries != 1 || !terminal.V2.DeadlineAt.Equal(state.DeadlineAt) {
		t.Fatalf("wrong resumed status: %s", raw)
	}
	if parents.Load() != 1 || children.Load() != 2 {
		t.Fatalf("requests parent=%d child=%d", parents.Load(), children.Load())
	}
	if terminal.V2.HTTPAttempts != 3 {
		t.Fatalf("durable attempts %d", terminal.V2.HTTPAttempts)
	}
	for _, suffix := range []string{"", "/chunks", "/tree", "/export"} {
		code, _ := v2ServiceRequest(t, base, "GET", path+suffix, "owner-two-a-long-random-looking-integration-token", nil)
		if code != 404 {
			t.Fatalf("owner leak %s %d", suffix, code)
		}
	}
	for _, suffix := range []string{"", "/result"} {
		code, _ := v2ServiceRequest(t, base, "GET", "/v1/executions/"+string(accepted.ID)+suffix, token, nil)
		if code != 404 {
			t.Fatalf("version leak %s %d", suffix, code)
		}
	}
	code, raw = v2ServiceRequest(t, base, "GET", path+"/chunks?after="+before.NextCursor, token, nil)
	var after domain.V2ChunkPage
	json.Unmarshal(raw, &after)
	if code != 200 || !after.Terminal || !bytes.Contains(raw, []byte(`9007199254740993`)) {
		t.Fatalf("resumed chunks %d %s", code, raw)
	}
	code, raw = v2ServiceRequest(t, base, "GET", path+"/export", token, nil)
	if code != 200 || !bytes.Contains(raw, []byte(`"type":"export_end"`)) {
		t.Fatalf("export %d %s", code, raw)
	}
	var delivery capturedWebhook
	var payload map[string]any
	var receivedNode, receivedTerminal bool
	webhookDeadline := time.After(5 * time.Second)
	for !receivedNode || !receivedTerminal {
		select {
		case incoming := <-deliveries:
			var event map[string]any
			if err := json.Unmarshal(incoming.Body, &event); err != nil {
				t.Fatal(err)
			}
			if event["type"] == "node.finished" {
				if event["status"] != "completed" || !bytes.Contains(incoming.Body, []byte(`9007199254740993`)) || event["eventId"] == "" {
					t.Fatalf("wrong node webhook %s", incoming.Body)
				}
				receivedNode = true
			} else {
				delivery, payload, receivedTerminal = incoming, event, true
			}
		case <-webhookDeadline:
			t.Fatalf("missing webhooks: node=%v terminal=%v", receivedNode, receivedTerminal)
		}
	}
	links := payload["links"].(map[string]any)
	if payload["id"] != string(accepted.ID) || payload["status"] != "completed" || payload["lastSequence"] != after.LastSequence || !strings.HasSuffix(links["chunks"].(string), path+"/chunks") {
		t.Fatalf("wrong final webhook %s", delivery.Body)
	}
	if _, ok := payload["result"]; ok {
		t.Fatal("v2 webhook embedded result")
	}
}
