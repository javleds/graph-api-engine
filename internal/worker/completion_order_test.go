package worker

import (
	"context"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	"testing"
	"time"
)

func TestChildrenCompleteInReverseWithoutReorderingCollection(t *testing.T) {
	second := make(chan struct{})
	fake := &fakeFetcher{handle: func(ctx context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		switch r.Request.URL {
		case "https://provider.test/items":
			return response(`[{"id":1},{"id":2}]`)
		case "https://provider.test/2":
			close(second)
			return response(`{"id":2}`)
		default:
			select {
			case <-second:
				return response(`{"id":1}`)
			case <-ctx.Done():
				return fetch.FetchResult{}, ctx.Err()
			}
		}
	}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	engine := Engine{Fetcher: fake, Options: DefaultOptions()}
	done := engine.Execute(ctx, compiled(call("items", "collection", "/items", call("detail", "single", "/{parent.id}"))), "owner", "id")
	if done.Status != execution.Completed {
		t.Fatal(done)
	}
	items := result(t, done)["items"].([]any)
	for i, item := range items {
		if item.(map[string]any)["detail"].(map[string]any)["id"] != float64(i+1) {
			t.Fatal(items)
		}
	}
}
