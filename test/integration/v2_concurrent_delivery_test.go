package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/javleds/api-engine/internal/api/http/dto"
	"github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/platform/config"
)

func TestV2ConcurrentSourceWebhooksWaitWhileAPIExposesFastChain(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	started := make(chan string, 16)
	unblock := func() { once.Do(func() { close(release) }) }
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- r.URL.Path
		switch r.URL.Path {
		case "/items":
			io.WriteString(w, `[{"id":1},{"id":2}]`)
		case "/detail/1":
			select {
			case <-release:
				io.WriteString(w, `{"species":"seed"}`)
			case <-r.Context().Done():
			}
		case "/detail/2":
			io.WriteString(w, `{"species":"flame"}`)
		case "/language":
			io.WriteString(w, `{"name":"en"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	defer unblock()
	deliveries := make(chan []byte, 8)
	acknowledge := make(chan struct{}, 8)
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		deliveries <- raw
		select {
		case <-acknowledge:
			w.WriteHeader(http.StatusAccepted)
		case <-r.Context().Done():
		}
	}))
	defer webhook.Close()
	cfg := fixtureConfig(t, upstream.URL, webhook.URL)
	cfg.Fetch.AttemptTimeout = config.Duration(3 * time.Second)
	cfg.Webhooks.PollInterval = config.Duration(5 * time.Millisecond)
	cfg.Webhooks.MaxPendingDeliveries = 1
	cfg.Executions.MaxBranchGoroutines = 4
	_, base, stop := startV2Service(t, cfg)
	defer func() { unblock(); stop() }()
	var input map[string]any
	if err := json.Unmarshal(definitionBody(upstream.URL, map[string]any{"url": webhook.URL, "ordering": "source", "maxInFlight": 1}), &input); err != nil {
		t.Fatal(err)
	}
	definition := input["definition"].(map[string]any)
	definition["api"].(map[string]any)["cache"] = false
	definition["calls"] = append(definition["calls"].([]any), map[string]any{"name": "language", "type": "single", "request": map[string]any{"method": "GET", "path": "/language"}})
	body, _ := json.Marshal(input)
	token := "owner-one-a-long-random-looking-integration-token"
	code, raw := v2ServiceRequest(t, base, "POST", "/v2/executions", token, body)
	if code != http.StatusAccepted {
		t.Fatalf("accept: %d %s", code, raw)
	}
	var accepted dto.CreateExecutionResponse
	if err := json.Unmarshal(raw, &accepted); err != nil {
		t.Fatal(err)
	}
	path := "/v2/executions/" + string(accepted.ID)
	deadline := time.Now().Add(10 * time.Second)
	var fast domain.V2Result
	for fast.NodeID == "" {
		code, raw = v2ServiceRequest(t, base, "GET", path+"/results", token, nil)
		var page domain.V2ResultPage
		if code != http.StatusOK || json.Unmarshal(raw, &page) != nil {
			t.Fatalf("results: %d %s", code, raw)
		}
		for _, result := range page.Results {
			if result.Root == "items" {
				if result.Index == nil || *result.Index != 1 {
					t.Fatalf("blocked first item unexpectedly completed: %+v", result)
				}
				fast = result
			}
		}
		if time.Now().After(deadline) {
			_, status := v2ServiceRequest(t, base, "GET", path, token, nil)
			var calls []string
			for len(started) > 0 {
				calls = append(calls, <-started)
			}
			t.Fatalf("fast second chain blocked: calls=%v status=%s results=%s", calls, status, raw)
		}
		if fast.NodeID == "" {
			time.Sleep(5 * time.Millisecond)
		}
	}
	code, raw = v2ServiceRequest(t, base, "GET", path+"/results/"+fast.NodeID, token, nil)
	var result completeNodeResult
	if code != http.StatusOK || json.Unmarshal(raw, &result) != nil || !result.Complete || !sameJSON(result.Data, []byte(`{"id":2,"detail":{"species":"flame"}}`)) {
		t.Fatalf("fast API chain incomplete: %d %s", code, raw)
	}
	select {
	case raw := <-deliveries:
		t.Fatalf("source webhook bypassed unfinished first chain: %s", raw)
	default:
	}
	unblock()
	expected := []string{`{"id":1,"detail":{"species":"seed"}}`, `{"id":2,"detail":{"species":"flame"}}`, `{"name":"en"}`}
	for index, want := range expected {
		raw := awaitConcurrentWebhook(t, deliveries)
		var event completedNodeWebhook
		if json.Unmarshal(raw, &event) != nil || event.Type != "node.finished" || !sameJSON(event.Result, []byte(want)) {
			t.Fatalf("delivery %d violated source order: %s", index, raw)
		}
		if index == 0 {
			deadline := time.Now().Add(3 * time.Second)
			for {
				code, raw := v2ServiceRequest(t, base, "GET", path+"/results", token, nil)
				var page domain.V2ResultPage
				if code != http.StatusOK || json.Unmarshal(raw, &page) != nil {
					t.Fatalf("API while webhook awaits acknowledgement: %d %s", code, raw)
				}
				if page.Terminal && len(page.Results) == 3 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("engine finalization waited for receiver acknowledgement")
				}
				time.Sleep(5 * time.Millisecond)
			}
		}
		// The receiver deliberately withholds its acknowledgement. Neither a later
		// source node nor the terminal event may use the occupied delivery window.
		select {
		case later := <-deliveries:
			t.Fatalf("delivery began before 2xx acknowledgement: %s", later)
		case <-time.After(30 * time.Millisecond):
		}
		acknowledge <- struct{}{}
	}
	raw = awaitConcurrentWebhook(t, deliveries)
	var terminal map[string]json.RawMessage
	if json.Unmarshal(raw, &terminal) != nil || string(terminal["type"]) != `"execution.completed"` || string(terminal["resultCount"]) != "3" {
		t.Fatalf("terminal did not follow all source deliveries: %s", raw)
	}
	acknowledge <- struct{}{}
}

func awaitConcurrentWebhook(t *testing.T, deliveries <-chan []byte) []byte {
	t.Helper()
	select {
	case raw := <-deliveries:
		return raw
	case <-time.After(5 * time.Second):
		t.Fatal("source notification queue stalled")
		return nil
	}
}
