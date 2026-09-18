package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestRootAuthorizationExhaustedRetriesAndAllChildrenFailure(t *testing.T) {
	for name, code := range map[string]int{"E05-unauthorized": 401, "E12-four-unavailable": 503, "E10-all-children": 404} {
		t.Run(name, func(t *testing.T) {
			var attempts atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if name == "E10-all-children" && r.URL.Path == "/items" {
					io.WriteString(w, `[{"id":1},{"id":2}]`)
					return
				}
				attempts.Add(1)
				w.WriteHeader(code)
				io.WriteString(w, `{}`)
			}))
			defer upstream.Close()
			fixture := openFixture(t, fixtureConfig(t, upstream.URL, upstream.URL))
			id := fixture.create(t, definitionBody(upstream.URL, nil), "")
			terminal := fixture.terminal(t, id)
			expectedStatus := "failed"
			expectedAttempts := int32(1)
			if name == "E12-four-unavailable" {
				expectedAttempts = 4
			}
			if name == "E10-all-children" {
				expectedStatus = "completed_with_errors"
				expectedAttempts = 2
			}
			if terminal["status"] != expectedStatus || attempts.Load() != expectedAttempts {
				t.Fatalf("terminal=%#v attempts=%d", terminal, attempts.Load())
			}
			status, data := fixture.request(t, "GET", "/v1/executions/"+id+"/result", fixture.token, "", nil)
			if expectedStatus == "failed" && status != 409 {
				t.Fatalf("failed result %d %s", status, data)
			}
			if expectedStatus == "completed_with_errors" {
				if status != 200 {
					t.Fatalf("partial result %d %s", status, data)
				}
				validateResultSchema(t, data)
			}
			if name == "E12-four-unavailable" {
				errors := terminal["errors"].([]any)
				entry := errors[0].(map[string]any)
				if len(errors) != 1 || entry["attempts"] != float64(4) || entry["retryExhausted"] != true {
					raw, _ := json.Marshal(terminal)
					t.Fatalf("wrong exhaustion report %s", raw)
				}
			}
		})
	}
}
