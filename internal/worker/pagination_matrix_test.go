package worker

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"testing"

	definition "github.com/javleds/api-engine/internal/definition/domain"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
)

func TestPaginationSizeAndStartMatrix(t *testing.T) {
	for name, sizes := range map[string][]int{"P01-short-final": {100, 100, 37}, "P02-empty-final": {100, 100, 0}, "P03-empty-first": {0}, "P04-inferred": {75, 75, 20}, "P06-early-stop": {100, 12}, "P07-zero-start": {100, 100, 12}} {
		t.Run(name, func(t *testing.T) {
			root := paginated()
			if name != "P04-inferred" {
				root.Pagination.PageSize = "size"
				root.Request.Query["size"] = 100
			}
			if name == "P06-early-stop" {
				root.Pagination.Pages = definition.PageSelection{Count: 5}
			}
			if name == "P07-zero-start" {
				root.Pagination.Start = 0
			}
			fake := &fakeFetcher{handle: func(_ context.Context, request fetch.FetchRequest) (fetch.FetchResult, error) {
				parsed, _ := url.Parse(request.Request.URL)
				page, _ := strconv.Atoi(parsed.Query().Get("page"))
				index := page - int(root.Pagination.Start)
				if index < 0 || index >= len(sizes) {
					t.Errorf("unexpected page %d", page)
					return response(`[]`)
				}
				values := make([]int, sizes[index])
				for i := range values {
					values[i] = index*1000 + i
				}
				data, _ := json.Marshal(values)
				return response(string(data))
			}}
			done := run(fake, root)
			expected := 0
			for _, size := range sizes {
				expected += size
			}
			if done.Status != execution.Completed || len(result(t, done)["items"].([]any)) != expected || len(fake.calls) != len(sizes) {
				t.Fatalf("unexpected pagination: %+v calls=%v", done, fake.calls)
			}
		})
	}
}
func TestPaginationChildrenHaveIndependentPageCounters(t *testing.T) {
	child := call("pages", "collection", "/items/{parent.id}/pages")
	child.Pagination = &definition.Pagination{Page: "page", Start: 1, Pages: definition.PageSelection{All: true}}
	root := call("items", "collection", "/items", child)
	fake := &fakeFetcher{handle: func(_ context.Context, request fetch.FetchRequest) (fetch.FetchResult, error) {
		parsed, _ := url.Parse(request.Request.URL)
		if parsed.Path == "/items" {
			return response(`[{"id":1},{"id":2}]`)
		}
		if parsed.Query().Get("page") == "1" {
			return response(`[1]`)
		}
		return response(`[]`)
	}}
	done := run(fake, root)
	if done.Status != execution.Completed {
		t.Fatal(done)
	}
	for _, id := range []string{"1", "2"} {
		for _, page := range []string{"1", "2"} {
			if fake.count("https://provider.test/items/"+id+"/pages?page="+page) != 1 {
				t.Fatal(fake.calls)
			}
		}
	}
	if len(fake.calls) != 5 {
		t.Fatal(fake.calls)
	}
}
