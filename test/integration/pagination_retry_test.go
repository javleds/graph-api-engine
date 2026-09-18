package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestPaginationRetryDoesNotDuplicateAcceptedPages(t *testing.T) {
	var secondPage atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("page") {
		case "1":
			io.WriteString(w, `[1,2]`)
		case "2":
			if secondPage.Add(1) == 1 {
				w.WriteHeader(503)
				io.WriteString(w, `{}`)
			} else {
				io.WriteString(w, `[3]`)
			}
		default:
			t.Error("requested unexpected page")
			io.WriteString(w, `[]`)
		}
	}))
	defer upstream.Close()
	fixture := openFixture(t, fixtureConfig(t, upstream.URL, upstream.URL))
	var input map[string]any
	json.Unmarshal(definitionBody(upstream.URL, nil), &input)
	definition := input["definition"].(map[string]any)
	root := definition["calls"].(map[string]any)
	delete(root, "children")
	root["pagination"] = map[string]any{"page": "page"}
	body, _ := json.Marshal(input)
	id := fixture.create(t, body, "")
	terminal := fixture.terminal(t, id)
	if terminal["status"] != "completed" {
		t.Fatal(terminal)
	}
	status, data := fixture.request(t, "GET", "/v1/executions/"+id+"/result", fixture.token, "", nil)
	if status != 200 {
		t.Fatalf("%d %s", status, data)
	}
	var result map[string]any
	json.Unmarshal(data, &result)
	items := result["result"].(map[string]any)["items"].([]any)
	if len(items) != 3 || items[0] != float64(1) || items[1] != float64(2) || items[2] != float64(3) || secondPage.Load() != 2 {
		t.Fatalf("duplicated or missing data: %s; attempts=%d", data, secondPage.Load())
	}
}
