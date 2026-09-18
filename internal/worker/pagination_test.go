package worker

import (
	"context"
	"fmt"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	"net/url"
	"testing"
)

func paginated() definition.CompiledCall {
	root := call("items", "collection", "/items")
	root.Pagination = &definition.Pagination{Page: "page", Start: 1, Pages: definition.PageSelection{All: true}}
	return root
}
func TestPaginationExplicitPartialFirstPageStops(t *testing.T) {
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) { return response(`[1,2]`) }}
	root := paginated()
	root.Pagination.PageSize = "size"
	root.Request.Query["size"] = 3
	done := run(fake, root)
	if done.Status != execution.Completed || len(fake.calls) != 1 {
		t.Fatal(done, fake.calls)
	}
}
func TestPaginationLargerPageDiscarded(t *testing.T) {
	fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		u, _ := url.Parse(r.Request.URL)
		if u.Query().Get("page") == "1" {
			return response(`[1,2]`)
		}
		return response(`[3,4,5]`)
	}}
	done := run(fake, paginated())
	if done.Report.Errors[0].Code != "PAGINATION_SIZE_INCONSISTENT" || len(result(t, done)["items"].([]any)) != 2 {
		t.Fatal(done)
	}
}
func TestPaginationFiniteCountIsCleanCompletion(t *testing.T) {
	fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		u, _ := url.Parse(r.Request.URL)
		return response(fmt.Sprintf(`[%s]`, u.Query().Get("page")))
	}}
	root := paginated()
	root.Pagination.Pages = definition.PageSelection{Count: 2}
	done := run(fake, root)
	if done.Status != execution.Completed || len(fake.calls) != 2 {
		t.Fatal(done, fake.calls)
	}
}
func TestPaginationOperationalGuardIsIncomplete(t *testing.T) {
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) { return response(`[1]`) }}
	engine := Engine{Fetcher: fake, Options: Options{MaxPagesPerCall: 2}}
	done := engine.Execute(context.Background(), compiled(paginated()), "owner", "id")
	if done.Status != execution.CompletedWithErrors || done.Report.Errors[0].Code != "PAGINATION_LIMIT_REACHED" || len(fake.calls) != 2 {
		t.Fatal(done, fake.calls)
	}
}
func TestPaginationRepeatedPageDiscarded(t *testing.T) {
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) { return response(`[1]`) }}
	done := run(fake, paginated())
	if done.Status != execution.CompletedWithErrors || done.Report.Errors[0].Code != "PAGINATION_NO_PROGRESS" || len(result(t, done)["items"].([]any)) != 1 {
		t.Fatal(done)
	}
}
func TestPaginationAbsoluteIndexesAcrossPages(t *testing.T) {
	fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		u, _ := url.Parse(r.Request.URL)
		if u.Query().Get("page") == "1" {
			return response(`[1,2]`)
		}
		if u.Query().Get("page") == "2" {
			return response(`[3]`)
		}
		return fetch.FetchResult{}, &fetch.UpstreamFailure{Code: "UPSTREAM_NOT_FOUND", Attempts: 1}
	}}
	root := paginated()
	root.Children = []definition.CompiledCall{call("details", "single", "/{parent.value}")}
	done := run(fake, root)
	if len(done.Report.Errors) != 3 || *done.Report.Errors[2].NodePath != "items[2].details" {
		t.Fatal(done.Report)
	}
}
