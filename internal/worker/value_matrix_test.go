package worker

import (
	"context"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	"testing"
)

func TestNodeTypeMismatchDoesNotCoerceValues(t *testing.T) {
	for kind, body := range map[string]string{"single": "[]", "collection": "{}"} {
		t.Run(kind, func(t *testing.T) {
			fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) { return response(body) }}
			done := run(fake, call("root", kind, "/root"))
			if done.Status != execution.Failed || len(done.Report.Errors) != 1 || done.Report.Errors[0].Code != "TYPE_MISMATCH" {
				t.Fatal(done)
			}
		})
	}
}
func TestChildlessScalarsRemainUnwrapped(t *testing.T) {
	for _, body := range []string{`7`, `"text"`, `true`, `null`} {
		fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) { return response(body) }}
		done := run(fake, call("root", "single", "/root"))
		if done.Status != execution.Completed {
			t.Fatal(done)
		}
		if _, wrapped := result(t, done)["root"].(map[string]any); wrapped {
			t.Fatal("unexpected value wrapper")
		}
	}
}
