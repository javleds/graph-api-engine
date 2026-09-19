package http_test

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlaygroundValidationDoesNotCreateExecutions(t *testing.T) {
	h, _, dispatcher := newTestAPI(t)
	dispatcher.busy = true
	valid := `{"definition":{"version":1,"api":{"baseUrl":"https://example.com","limits":{"requests":100,"per":"1m"}},"calls":{"name":"items","type":"collection","request":{"method":"GET","path":"/items"}}}}`
	for _, body := range []string{valid, strings.Replace(valid, `"path":"/items"`, `"path":"/items","query":{"id":9007199254740993}`, 1)} {
		got := request(t, h, "POST", "/playground/api/validate", []byte(body), "", "")
		if got.Code != 200 || got.Body.String() != `{"valid":true}` {
			t.Fatalf("%d %s", got.Code, got.Body)
		}
	}
	if dispatcher.commits != 0 {
		t.Fatal("validation dispatched execution")
	}
	for _, body := range []string{`{}`, strings.Replace(valid, `"path":"/items"`, `"path":"/items/{parent.id}"`, 1), strings.Replace(valid, `"name":"items"`, `"name":"items","unknown":true`, 1)} {
		got := request(t, h, "POST", "/playground/api/validate", []byte(body), "", "")
		if got.Code != 422 {
			t.Fatalf("%d %s", got.Code, got.Body)
		}
	}
	for _, body := range []string{`{`, strings.Replace(valid, `"version":1`, `"version":1,"version":1`, 1)} {
		if got := request(t, h, "POST", "/playground/api/validate", []byte(body), "", ""); got.Code != 400 {
			t.Fatal(got.Code)
		}
	}
	if got := request(t, h, "POST", "/playground/api/validate", []byte(strings.Repeat(" ", 262145)), "", ""); got.Code != 413 {
		t.Fatal(got.Code)
	}
	r := httptest.NewRequest("POST", "/playground/api/validate", bytes.NewBufferString(valid))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 415 {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest("POST", "http://localhost/playground/api/validate", bytes.NewBufferString(valid))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://other.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	if dispatcher.commits != 0 {
		t.Fatal("invalid input dispatched execution")
	}
}
