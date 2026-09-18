package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	"net/url"
	"strings"
	"testing"
	"time"
)

func call(name, kind, path string, children ...definition.CompiledCall) definition.CompiledCall {
	names := []string{"root", "items", "games"}
	template, err := definition.CompileTemplate(path, names)
	if err != nil {
		panic(err)
	}
	return definition.CompiledCall{Name: name, Type: kind, Pointer: "/definition/calls/" + name, Request: definition.RequestTemplate{Method: "GET", Path: template, Query: map[string]any{}}, Children: children}
}
func compiled(roots ...definition.CompiledCall) definition.CompiledDefinition {
	return definition.CompiledDefinition{Version: 1, API: definition.APIConnection{BaseURL: "https://provider.test", Limits: definition.RatePolicy{Requests: 100, Per: time.Second, Concurrency: 8}}, Roots: roots}
}
func run(fake *fakeFetcher, roots ...definition.CompiledCall) execution.ExecutionCompletion {
	engine := Engine{Fetcher: fake, Options: DefaultOptions()}
	return engine.Execute(context.Background(), compiled(roots...), "owner", "test-id")
}
func result(t *testing.T, completion execution.ExecutionCompletion) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(completion.Data, &value); err != nil {
		t.Fatalf("result absent: %+v, %v", completion, err)
	}
	return value
}
func TestChildFailurePreservesSiblingsAndSkipsDescendants(t *testing.T) {
	fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		switch r.Request.URL {
		case "https://provider.test/items":
			return response(`[{"id":1},{"id":2}]`)
		case "https://provider.test/1/games":
			return fetch.FetchResult{}, &fetch.UpstreamFailure{Code: "UPSTREAM_FORBIDDEN", HTTPStatus: 403, Attempts: 1}
		case "https://provider.test/2/games":
			return response(`[{"id":10}]`)
		default:
			return response(`{"ok":true}`)
		}
	}}
	root := call("items", "collection", "/items", call("details", "single", "/{parent.id}/details"), call("games", "collection", "/{parent.id}/games", call("more", "single", "/games/{parent.id}")))
	done := run(fake, root)
	if done.Status != execution.CompletedWithErrors || done.Report.ErrorSummary.Total != 1 {
		t.Fatalf("unexpected outcome %+v", done)
	}
	if *done.Report.Errors[0].NodePath != "items[0].games" || !done.Report.Errors[0].DescendantsSkipped {
		t.Fatal(done.Report.Errors)
	}
	items := result(t, done)["items"].([]any)
	first := items[0].(map[string]any)
	if len(first["games"].([]any)) != 0 || first["details"] == nil {
		t.Fatal(first)
	}
	if fake.count("https://provider.test/games/10") != 1 || fake.count("https://provider.test/2/details") != 1 {
		t.Fatal(fake.calls)
	}
	if len(fake.calls) != 6 {
		t.Fatalf("descendants should not be invented: %#v", fake.calls)
	}
}
func TestAllRootsFailWithoutPublishingPlaceholders(t *testing.T) {
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) {
		return fetch.FetchResult{}, &fetch.UpstreamFailure{Code: "UPSTREAM_UNAUTHORIZED", HTTPStatus: 401, Attempts: 1}
	}}
	done := run(fake, call("a", "single", "/a"), call("b", "collection", "/b"))
	if done.Status != execution.Failed || done.Error.Code != "ALL_ROOT_CALLS_FAILED" || len(done.Data) != 0 || done.Report.ErrorSummary.Total != 2 {
		t.Fatal(done)
	}
}
func TestEmptyRootIsAvailableDespiteAnotherRootFailure(t *testing.T) {
	fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		if strings.HasSuffix(r.Request.URL, "/a") {
			return response(`[]`)
		}
		return fetch.FetchResult{}, &fetch.UpstreamFailure{Code: "UPSTREAM_NOT_FOUND", Attempts: 1}
	}}
	done := run(fake, call("a", "collection", "/a"), call("b", "single", "/b"))
	if done.Status != execution.CompletedWithErrors || !done.Report.Roots[0].DataAvailable {
		t.Fatal(done)
	}
}
func TestNullAndEmptyAreValidOwnData(t *testing.T) {
	fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		if strings.HasSuffix(r.Request.URL, "/a") {
			return response(`null`)
		}
		return response(`[]`)
	}}
	done := run(fake, call("a", "single", "/a"), call("b", "collection", "/b", call("child", "single", "/never")))
	if done.Status != execution.Completed || done.Report.ErrorSummary.Total != 0 || fake.count("https://provider.test/never") != 0 {
		t.Fatal(done)
	}
}
func TestCompositionKeepsPositionsAndFreezesAncestorContext(t *testing.T) {
	fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		if strings.HasSuffix(r.Request.URL, "/root") {
			return response(`[{"id":1,"details":"collision"},null,[1],7,{"id":5}]`)
		}
		return response(`{"yes":true}`)
	}}
	root := call("root", "collection", "/root", call("details", "single", "/ok"))
	done := run(fake, root)
	items := result(t, done)["root"].([]any)
	if len(items) != 5 || items[0] != nil || items[1] != nil || items[2] != nil || items[3].(map[string]any)["value"] != float64(7) {
		t.Fatal(items)
	}
	if done.Report.ErrorSummary.Total != 3 || fake.count("https://provider.test/ok") != 2 {
		t.Fatal(done.Report, fake.calls)
	}
}
func TestBindingFailureDoesNotFetchAndSiblingContinues(t *testing.T) {
	fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		if strings.HasSuffix(r.Request.URL, "/root") {
			return response(`[{"id":1},{}]`)
		}
		return response(`true`)
	}}
	done := run(fake, call("root", "collection", "/root", call("details", "single", "/{parent.id}"), call("other", "single", "/other")))
	if done.Status != execution.CompletedWithErrors || done.Report.Errors[0].Attempts != 0 || done.Report.Errors[0].Code != "BINDING_PATH_NOT_FOUND" || fake.count("https://provider.test/other") != 2 {
		t.Fatal(done, fake.calls)
	}
}
func TestPaginationRetainsPrefixAndStopsAtFailedPage(t *testing.T) {
	fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		u, _ := url.Parse(r.Request.URL)
		switch u.Query().Get("page") {
		case "1":
			return response(`[1,2]`)
		case "2":
			return response(`[3,4]`)
		default:
			return fetch.FetchResult{}, &fetch.UpstreamFailure{Code: "UPSTREAM_UNAVAILABLE", HTTPStatus: 503, Attempts: 4, Retryable: true, RetryExhausted: true}
		}
	}}
	root := call("items", "collection", "/items")
	root.Pagination = &definition.Pagination{Page: "page", Start: 1, Pages: definition.PageSelection{All: true}}
	done := run(fake, root)
	if len(result(t, done)["items"].([]any)) != 4 || *done.Report.Errors[0].Page != 3 || fake.count("https://provider.test/items?page=4") != 0 {
		t.Fatal(done)
	}
}
func TestPaginationSelectFailureIsNotEmptySuccess(t *testing.T) {
	fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		if strings.Contains(r.Request.URL, "page=1") {
			return response(`{"items":[1]}`)
		}
		return response(`{}`)
	}}
	root := call("items", "collection", "/items")
	root.Select = []string{"items"}
	root.Pagination = &definition.Pagination{Page: "page", Start: 1, Pages: definition.PageSelection{All: true}}
	done := run(fake, root)
	if done.Status != execution.CompletedWithErrors || done.Report.Errors[0].Code != "SELECT_PATH_NOT_FOUND" || len(result(t, done)["items"].([]any)) != 1 {
		t.Fatal(done)
	}
}
func TestPaginationChildFailureDoesNotStopNextPage(t *testing.T) {
	fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		if strings.Contains(r.Request.URL, "page=1") {
			return response(`[1,2]`)
		}
		if strings.Contains(r.Request.URL, "page=2") {
			return response(`[]`)
		}
		return fetch.FetchResult{}, &fetch.UpstreamFailure{Code: "UPSTREAM_FORBIDDEN", Attempts: 1}
	}}
	root := call("items", "collection", "/items", call("details", "single", "/details/{parent.value}"))
	root.Pagination = &definition.Pagination{Page: "page", Start: 1, Pages: definition.PageSelection{All: true}}
	done := run(fake, root)
	if done.Report.ErrorSummary.Total != 2 || fake.count("https://provider.test/items?page=2") != 1 {
		t.Fatal(done)
	}
}
func TestOneBranchSlotDoesNotDeadlockNestedChildren(t *testing.T) {
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) { return response(`{"id":1}`) }}
	root := call("deep", "single", "/deep")
	for depth := 0; depth < 20; depth++ {
		root = call(fmt.Sprintf("n%d", depth), "single", "/deep", root)
	}
	engine := Engine{Fetcher: fake, Options: Options{MaxBranchGoroutines: 1, Timeout: time.Second}}
	done := engine.Execute(context.Background(), compiled(root), "owner", "id")
	if done.Status != execution.Completed {
		t.Fatal(done)
	}
}
func TestPanicIsNotPublishablePartial(t *testing.T) {
	fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		if strings.HasSuffix(r.Request.URL, "/panic") {
			panic("secret")
		}
		return response(`{}`)
	}}
	done := run(fake, call("a", "single", "/ok"), call("b", "single", "/panic"))
	if done.Status != execution.Failed || done.Error.Code != "INTERNAL_WORKER_PANIC" || len(done.Data) != 0 {
		t.Fatal(done)
	}
}
func TestLogicalBudgetProducesOneGlobalDiagnostic(t *testing.T) {
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) { return response(`{"id":1}`) }}
	engine := Engine{Fetcher: fake, Options: Options{MaxLogicalRequests: 1, MaxBranchGoroutines: 1}}
	done := engine.Execute(context.Background(), compiled(call("root", "single", "/root", call("child", "single", "/child"))), "owner", "id")
	if done.Status != execution.CompletedWithErrors || done.Report.ErrorSummary.Total != 1 || done.Report.Errors[0].Scope != "execution" {
		t.Fatal(done)
	}
}
func TestEnvelopeHardLimitDoesNotPublishTruncatedJSON(t *testing.T) {
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) { return response(`{"a":"123"}`) }}
	engine := Engine{Fetcher: fake, Options: Options{MaxResultBytes: 8}}
	done := engine.Execute(context.Background(), compiled(call("root", "single", "/root")), "owner", "id")
	if done.Status != execution.Failed || done.Error.Code != "RESULT_TOO_LARGE" || len(done.Data) != 0 {
		t.Fatal(done)
	}
}
func TestGlobalDeadlineWithoutRootsUsesGlobalFailure(t *testing.T) {
	fake := &fakeFetcher{handle: func(ctx context.Context, _ fetch.FetchRequest) (fetch.FetchResult, error) {
		<-ctx.Done()
		return fetch.FetchResult{}, ctx.Err()
	}}
	engine := Engine{Fetcher: fake, Options: Options{Timeout: time.Millisecond}}
	done := engine.Execute(context.Background(), compiled(call("root", "single", "/root")), "owner", "id")
	if done.Status != execution.Failed || done.Error.Code != "EXECUTION_TIMEOUT" || done.Report.ErrorSummary.Total != 1 {
		t.Fatal(done)
	}
}
func TestGlobalDeadlineKeepsValidParentAndPlaceholders(t *testing.T) {
	fake := &fakeFetcher{handle: func(ctx context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		if strings.HasSuffix(r.Request.URL, "/root") {
			return response(`{"id":1}`)
		}
		<-ctx.Done()
		return fetch.FetchResult{}, errors.New("canceled")
	}}
	engine := Engine{Fetcher: fake, Options: Options{Timeout: time.Millisecond * 20}}
	done := engine.Execute(context.Background(), compiled(call("root", "single", "/root", call("child", "single", "/child"))), "owner", "id")
	if done.Status != execution.CompletedWithErrors || done.Report.ErrorSummary.Total != 1 || result(t, done)["root"].(map[string]any)["child"] != nil {
		t.Fatal(done)
	}
}

func TestSharedFailureKeepsEachConsumersLocation(t *testing.T) {
	shared := &fetch.UpstreamFailure{Code: "UPSTREAM_NOT_FOUND", HTTPStatus: 404, Attempts: 1}
	fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		if strings.HasSuffix(r.Request.URL, "/root") {
			return response(`[1,1]`)
		}
		return fetch.FetchResult{}, shared
	}}
	done := run(fake, call("root", "collection", "/root", call("details", "single", "/{parent.value}")))
	if len(done.Report.Errors) != 2 || *done.Report.Errors[0].NodePath != "root[0].details" || *done.Report.Errors[1].NodePath != "root[1].details" {
		t.Fatal(done.Report)
	}
}
func TestSharedResponseBodyIsDecodedPerConsumer(t *testing.T) {
	body := []byte(`{"present":1}`)
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) {
		return fetch.FetchResult{Response: fetch.RawResponse{Body: body}}, nil
	}}
	a := call("a", "single", "/same")
	a.Select = []string{"missing"}
	b := call("b", "single", "/same")
	b.Select = []string{"present"}
	done := run(fake, a, b)
	if done.Status != execution.CompletedWithErrors || done.Report.ErrorSummary.Total != 1 || result(t, done)["b"] != float64(1) {
		t.Fatal(done)
	}
}
func TestHTTPAttemptBudgetStopsExpansion(t *testing.T) {
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) { return response(`{"id":1}`) }}
	engine := Engine{Fetcher: fake, Options: Options{MaxHTTPAttempts: 1}}
	done := engine.Execute(context.Background(), compiled(call("root", "single", "/root", call("details", "single", "/details"))), "owner", "id")
	if done.Status != execution.CompletedWithErrors || done.Progress.HTTPAttempts != 1 || done.Report.Errors[0].Code != "EXECUTION_BUDGET_EXCEEDED" || fake.count("https://provider.test/details") != 0 {
		t.Fatal(done)
	}
}
func TestTypedTransportPanicRemainsTechnicalFailure(t *testing.T) {
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) {
		return fetch.FetchResult{}, &fetch.UpstreamFailure{Code: "INTERNAL_WORKER_PANIC"}
	}}
	done := run(fake, call("root", "single", "/root"))
	if done.Status != execution.Failed || done.Error.Code != "INTERNAL_WORKER_PANIC" || done.Report.ErrorSummary.Total != 0 {
		t.Fatal(done)
	}
}

func TestLogicalFetchDeadlineIsLocalTimeout(t *testing.T) {
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) {
		return fetch.FetchResult{}, context.DeadlineExceeded
	}}
	done := run(fake, call("root", "single", "/root"))
	if done.Report.Errors[0].Code != "UPSTREAM_TIMEOUT" || done.Error.Code != "ALL_ROOT_CALLS_FAILED" {
		t.Fatal(done)
	}
}
func TestUnknownFetchErrorIsTechnicalAndSanitized(t *testing.T) {
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) {
		return fetch.FetchResult{}, errors.New("unexpected secret URL")
	}}
	done := run(fake, call("root", "single", "/root"))
	if done.Status != execution.Failed || done.Error.Code != "INTERNAL_WORKER_ERROR" || strings.Contains(done.Error.Message, "secret") || done.Report.ErrorSummary.Total != 0 {
		t.Fatal(done)
	}
}

func TestProgressDistinguishesSuccessfulPartialAndFailedRoots(t *testing.T) {
	fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		if strings.HasSuffix(r.Request.URL, "/good") || strings.HasSuffix(r.Request.URL, "/partial") {
			return response(`{"id":1}`)
		}
		return fetch.FetchResult{}, &fetch.UpstreamFailure{Code: "UPSTREAM_NOT_FOUND", Attempts: 1}
	}}
	done := run(fake, call("good", "single", "/good"), call("partial", "single", "/partial", call("child", "single", "/child")), call("failed", "single", "/failed"))
	if done.Progress.SuccessfulRootCalls != 1 || done.Progress.PartialRootCalls != 1 || done.Progress.FailedRootCalls != 1 {
		t.Fatal(done.Progress)
	}
}
