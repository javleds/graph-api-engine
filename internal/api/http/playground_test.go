package http_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestPlaygroundOpenRoutesRemainIsolated(t *testing.T) {
	h, _, _ := newTestAPI(t)
	body, err := os.ReadFile("../../../contracts/examples/pokemon_full.json")
	if err != nil {
		t.Fatal(err)
	}
	normal := request(t, h, "POST", "/v1/executions", body, "", "owner")
	var original map[string]any
	if err := json.Unmarshal(normal.Body.Bytes(), &original); err != nil {
		t.Fatal(err)
	}
	created := request(t, h, "POST", "/playground/api/executions", body, "demo", "")
	if created.Code != 202 {
		t.Fatalf("%d %s", created.Code, created.Body)
	}
	var accepted map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	self := "/playground/api/executions/" + accepted["id"].(string)
	if created.Header().Get("Location") != self || accepted["links"].(map[string]any)["self"] != self {
		t.Fatal(created.Body)
	}
	if got := request(t, h, "GET", self, nil, "", ""); got.Code != 200 || !strings.Contains(got.Body.String(), self) {
		t.Fatal(got.Body)
	}
	if got := request(t, h, "GET", self+"/result", nil, "", ""); got.Code != 409 {
		t.Fatal(got.Code)
	}
	if got := request(t, h, "GET", "/playground/api/executions/"+original["id"].(string), nil, "", "owner"); got.Code != 404 {
		t.Fatal(got.Code)
	}
	if got := request(t, h, "GET", "/v1/executions/"+accepted["id"].(string), nil, "", "owner"); got.Code != 404 {
		t.Fatal(got.Code)
	}
	if got := request(t, h, "GET", "/v1/executions/"+accepted["id"].(string), nil, "", ""); got.Code != 401 {
		t.Fatal(got.Code)
	}
	replay := request(t, h, "POST", "/playground/api/executions", body, "demo", "")
	if replay.Code != 202 || replay.Header().Get("Location") != self {
		t.Fatal(replay.Body)
	}
}

func TestPlaygroundCrossOriginAndPage(t *testing.T) {
	h, _, _ := newTestAPI(t)
	for _, origin := range []string{"https://evil.example", "null"} {
		r := httptest.NewRequest("POST", "http://engine.example/playground/api/executions", bytes.NewBufferString(`{}`))
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("origin %s: %d", origin, w.Code)
		}
	}
	r := httptest.NewRequest("POST", "http://engine.example/playground/api/executions", bytes.NewBufferString(`{}`))
	r.Header.Set("Origin", "http://engine.example")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 422 {
		t.Fatalf("same origin: %d", w.Code)
	}
	page := request(t, h, "GET", "/playground", nil, "", "")
	if page.Code != 200 || !strings.Contains(page.Header().Get("Content-Type"), "text/html") || !strings.Contains(page.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatal(page)
	}
	if page.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(page.Header())
	}
	if got := request(t, h, "GET", "/playground/unknown", nil, "", ""); got.Code != 404 {
		t.Fatal(got.Code)
	}
}

func TestPlaygroundAssetsAreExplicitAndEmbedded(t *testing.T) {
	h, _, _ := newTestAPI(t)
	for _, name := range []string{"playground.css", "playground.js"} {
		got := request(t, h, "GET", "/playground/assets/"+name, nil, "", "")
		if got.Code != 200 || got.Body.Len() == 0 || got.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s: %d", name, got.Code)
		}
	}
	for _, name := range []string{"index.html", "assets.go", "unknown.js"} {
		if got := request(t, h, "GET", "/playground/assets/"+name, nil, "", ""); got.Code != 404 {
			t.Fatalf("exposed %s", name)
		}
	}
}
