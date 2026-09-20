package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/javleds/api-engine/internal/api/http/dto"
	"github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/platform/config"
)

func TestCancelStopsUpstreamAndPersistsAcrossRestart(t *testing.T) {
	for _, version := range []string{"v1", "v2"} {
		t.Run(version, func(t *testing.T) {
			started := make(chan struct{}, 1)
			stopped := make(chan struct{}, 1)
			var childCalls, descendants atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/items":
					io.WriteString(w, `[{"id":7}]`)
				case strings.HasPrefix(r.URL.Path, "/detail/"):
					childCalls.Add(1)
					select {
					case started <- struct{}{}:
					default:
					}
					<-r.Context().Done()
					select {
					case stopped <- struct{}{}:
					default:
					}
				default:
					descendants.Add(1)
					io.WriteString(w, `true`)
				}
			}))
			t.Cleanup(upstream.Close)
			cfg := fixtureConfig(t, upstream.URL, upstream.URL)
			cfg.Fetch.AttemptTimeout = config.Duration(2 * time.Second)
			app, base, stop := startV2Service(t, cfg)
			var input map[string]any
			json.Unmarshal(definitionBody(upstream.URL, nil), &input)
			definition := input["definition"].(map[string]any)
			definition["api"].(map[string]any)["cache"] = false
			child := definition["calls"].([]any)[0].(map[string]any)["children"].([]any)[0].(map[string]any)
			child["children"] = []any{map[string]any{"name": "never", "type": "single", "request": map[string]any{"method": "GET", "path": "/never"}}}
			body, _ := json.Marshal(input)
			token := "owner-one-a-long-random-looking-integration-token"
			code, raw := v2ServiceRequest(t, base, "POST", "/"+version+"/executions", token, body)
			if code != 202 {
				t.Fatalf("create %d %s", code, raw)
			}
			var accepted dto.CreateExecutionResponse
			json.Unmarshal(raw, &accepted)
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("upstream never started")
			}
			path := "/" + version + "/executions/" + string(accepted.ID)
			if version == "v2" {
				code, raw = v2ServiceRequest(t, base, "GET", path+"/chunks", token, nil)
				if code != 200 || !strings.Contains(string(raw), `"type":"data"`) {
					t.Fatalf("missing progressive data %d %s", code, raw)
				}
			}
			code, raw = v2ServiceRequest(t, base, "POST", path+"/cancel", token, nil)
			var cancelled dto.CancelExecutionResponse
			json.Unmarshal(raw, &cancelled)
			if code != 200 || cancelled.Status != domain.Failed || cancelled.Error == nil || cancelled.Error.Code != "EXECUTION_CANCELLED" {
				t.Fatalf("cancel %d %s", code, raw)
			}
			select {
			case <-stopped:
			case <-time.After(time.Second):
				t.Fatal("server-side HTTP request was not canceled")
			}
			wait, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := app.Dispatcher.Wait(wait); err != nil {
				t.Fatalf("worker failed to stop: %v", err)
			}
			if code, raw = v2ServiceRequest(t, base, "GET", "/health/ready", "", nil); code != 200 {
				t.Fatalf("cancel degraded health %d %s", code, raw)
			}
			if childCalls.Load() != 1 || descendants.Load() != 0 {
				t.Fatalf("continued work children=%d descendants=%d", childCalls.Load(), descendants.Load())
			}
			if code, raw = v2ServiceRequest(t, base, "POST", path+"/cancel", token, nil); code != 200 {
				t.Fatalf("repeat %d %s", code, raw)
			}
			if version == "v2" {
				code, raw = v2ServiceRequest(t, base, "GET", path+"/chunks", token, nil)
				var page domain.V2ChunkPage
				json.Unmarshal(raw, &page)
				if code != 200 || !page.Terminal || len(page.Chunks) < 2 || !strings.Contains(string(page.Chunks[len(page.Chunks)-1]), "EXECUTION_CANCELLED") {
					t.Fatalf("chunks %d %s", code, raw)
				}
			}
			stop()
			_, base, _ = startV2Service(t, cfg)
			code, raw = v2ServiceRequest(t, base, "GET", path, token, nil)
			if code != 200 || !strings.Contains(string(raw), "EXECUTION_CANCELLED") {
				t.Fatalf("cancel lost on restart %d %s", code, raw)
			}
			if childCalls.Load() != 1 {
				t.Fatal("cancelled execution restarted")
			}
		})
	}
}
