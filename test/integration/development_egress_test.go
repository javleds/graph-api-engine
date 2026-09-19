package integration

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestDevelopmentAllowsUnlistedProviderAndWebhook(t *testing.T) {
	var calls, deliveries atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[]`)
	}))
	defer upstream.Close()
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { deliveries.Add(1); w.WriteHeader(204) }))
	defer receiver.Close()
	cfg := fixtureConfig(t, "https://listed.example", "https://listed-webhook.example")
	cfg.Security.Egress.AllowAnyOriginForDevelopment = true
	f := openFixture(t, cfg)
	body := definitionBody(upstream.URL, map[string]any{"url": receiver.URL})
	if status, data := f.request(t, "POST", "/playground/api/validate", "", "", body); status != 200 {
		t.Fatalf("validation %d %s", status, data)
	}
	id := f.create(t, body, "")
	if terminal := f.terminal(t, id); terminal["status"] != "completed" {
		t.Fatal(terminal)
	}
	if err := f.app.Notifications.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || deliveries.Load() != 1 {
		t.Fatalf("upstream=%d webhook=%d", calls.Load(), deliveries.Load())
	}
}
