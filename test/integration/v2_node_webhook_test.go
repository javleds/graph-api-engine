package integration

import (
	"bytes"
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

func TestV2NodeWebhookPublishesCompleteChainBeforeNextChild(t *testing.T) {
	blocked := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/language":
			io.WriteString(w, `{"name":"en"}`)
		case "/items":
			io.WriteString(w, `[{"id":1,"name":"bulbasaur"},{"id":2,"name":"charizard"}]`)
		case "/detail/1":
			io.WriteString(w, `{"species":"seed","number":9007199254740993}`)
		case "/detail/2":
			close(blocked)
			select {
			case <-release:
				io.WriteString(w, `{"species":"flame"}`)
			case <-r.Context().Done():
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	defer unblock()
	deliveries := make(chan []byte, 16)
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		deliveries <- raw
		w.WriteHeader(http.StatusNoContent)
	}))
	defer webhook.Close()
	cfg := fixtureConfig(t, upstream.URL, webhook.URL)
	cfg.Fetch.AttemptTimeout = config.Duration(15 * time.Second)
	_, base, stop := startV2Service(t, cfg)
	defer func() { unblock(); stop() }()
	var input map[string]any
	if err := json.Unmarshal(definitionBody(upstream.URL, map[string]any{"url": webhook.URL, "payload": "status"}), &input); err != nil {
		t.Fatal(err)
	}
	definition := input["definition"].(map[string]any)
	definition["api"].(map[string]any)["cache"] = false
	definition["calls"] = append([]any{map[string]any{"name": "language", "type": "single", "request": map[string]any{"method": "GET", "path": "/language"}}}, definition["calls"].([]any)...)
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
	select {
	case <-blocked:
	case <-time.After(20 * time.Second):
		t.Fatal("second child did not start")
	}
	code, raw = v2ServiceRequest(t, base, "GET", path+"/results", token, nil)
	var page domain.V2ResultPage
	if code != http.StatusOK || json.Unmarshal(raw, &page) != nil || len(page.Results) != 2 || page.Terminal || page.Results[0].Root != "language" || page.Results[1].Root != "items" {
		t.Fatalf("expected two closed chains with second child pending: %d %s", code, raw)
	}
	expected := json.RawMessage(`{"id":1,"name":"bulbasaur","detail":{"species":"seed","number":9007199254740993}}`)
	code, raw = v2ServiceRequest(t, base, "GET", path+"/results/"+page.Results[1].NodeID, token, nil)
	var result completeNodeResult
	if code != http.StatusOK || json.Unmarshal(raw, &result) != nil || !result.Complete || result.EventID != page.Results[1].EventID || !sameJSON(result.Data, expected) {
		t.Fatalf("complete result: %d %s", code, raw)
	}
	seen := map[string]bool{}
	deadline := time.After(5 * time.Second)
	for len(seen) < 2 {
		select {
		case raw := <-deliveries:
			var event completedNodeWebhook
			if json.Unmarshal(raw, &event) != nil || event.Type != "node.finished" || event.Status != "completed" {
				t.Fatalf("premature terminal or invalid node: %s", raw)
			}
			if event.EventID == result.EventID && !sameJSON(event.Result, expected) {
				t.Fatalf("webhook chain differs from API: %s", raw)
			}
			seen[event.EventID] = true
		case <-deadline:
			t.Fatalf("expected node webhooks before release, got %d", len(seen))
		}
	}
	if !seen[result.EventID] || !seen[page.Results[0].EventID] {
		t.Fatalf("webhooks do not share result identities: %v", seen)
	}
	unblock()
	terminalSeen := false
	deadline = time.After(5 * time.Second)
	for len(seen) < 3 || !terminalSeen {
		select {
		case raw := <-deliveries:
			var event map[string]json.RawMessage
			if err := json.Unmarshal(raw, &event); err != nil {
				t.Fatal(err)
			}
			if string(event["type"]) == `"node.finished"` {
				var id string
				json.Unmarshal(event["eventId"], &id)
				seen[id] = true
			} else {
				if string(event["resultCount"]) != "3" || string(event["lastResultSequence"]) != `"3"` || string(event["status"]) != `"completed"` {
					t.Fatalf("wrong terminal: %s", raw)
				}
				terminalSeen = true
			}
		case <-deadline:
			t.Fatalf("webhook drain: nodes=%d terminal=%v", len(seen), terminalSeen)
		}
	}
}

func sameJSON(left, right []byte) bool {
	decode := func(raw []byte) ([]byte, error) {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		var value any
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		return json.Marshal(value)
	}
	a, errA := decode(left)
	b, errB := decode(right)
	return errA == nil && errB == nil && bytes.Equal(a, b)
}
