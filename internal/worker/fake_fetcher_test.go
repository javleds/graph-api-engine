package worker

import (
	"context"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	"sync"
)

type fakeFetcher struct {
	mu     sync.Mutex
	calls  map[string]int
	handle func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error)
}

func (f *fakeFetcher) Fetch(ctx context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
	if r.BeforeAttempt != nil {
		if err := r.BeforeAttempt(ctx); err != nil {
			return fetch.FetchResult{}, err
		}
	}
	if r.OnAttempt != nil {
		r.OnAttempt()
	}
	f.mu.Lock()
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[r.Request.URL]++
	f.mu.Unlock()
	return f.handle(ctx, r)
}
func (f *fakeFetcher) count(url string) int { f.mu.Lock(); defer f.mu.Unlock(); return f.calls[url] }
func response(body string) (fetch.FetchResult, error) {
	return fetch.FetchResult{Response: fetch.RawResponse{StatusCode: 200, Body: []byte(body), Attempts: 1}}, nil
}
