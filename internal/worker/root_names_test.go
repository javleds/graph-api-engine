package worker

import (
	"context"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	"testing"
)

func TestRootNamesErrorsAndStatusRemainInsideData(t *testing.T) {
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) {
		return response(`{"business":true}`)
	}}
	done := run(fake, call("errors", "single", "/errors"), call("status", "single", "/status"))
	if done.Status != execution.Completed || done.Report.ErrorSummary.Total != 0 {
		t.Fatal(done)
	}
	data := result(t, done)
	if data["errors"] == nil || data["status"] == nil {
		t.Fatal(data)
	}
}
