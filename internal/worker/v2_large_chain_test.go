package worker

import (
	"context"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	"io"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestV2LargeSingleChainStreamsWithBoundedMemory(t *testing.T) {
	pages := 24
	if os.Getenv("ENGINE_V2_LARGE") == "1" {
		pages = 256
	}
	const scalarBytes = 64 << 10
	body := `["` + strings.Repeat("x", scalarBytes) + `"` + strings.Repeat(`,"`+strings.Repeat("x", scalarBytes)+`"`, 7) + `]`
	child := call("items", "collection", "/items")
	child.Pagination = &definition.Pagination{Page: "page", Start: 1, Pages: definition.PageSelection{Count: int64(pages)}}
	fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		if strings.HasSuffix(r.Request.URL, "/root") {
			return response(`{"id":1}`)
		}
		page := r.Request.URL[strings.LastIndex(r.Request.URL, "=")+1:]
		return response(strings.Replace(body, `["`, `["`+page+":", 1))
	}}
	runner, store := durableResultRunner(t, fake, call("root", "single", "/root", child))
	runner.Engine.Options.MaxPagesPerCall = pages + 1
	if err := runner.Run(context.Background(), "results"); err != nil {
		t.Fatal(err)
	}
	page, err := store.ReadV2Results(context.Background(), "owner", "results", 0, 0, 100, time.Now())
	if err != nil || len(page.Results) != 1 {
		t.Fatalf("one complete chain expected: %+v %v", page, err)
	}
	var peak atomic.Uint64
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				var memory runtime.MemStats
				runtime.ReadMemStats(&memory)
				if memory.HeapAlloc > peak.Load() {
					peak.Store(memory.HeapAlloc)
				}
			}
		}
	}()
	started := time.Now()
	err = store.WriteV2Result(context.Background(), "owner", "results", page.Results[0].NodeID, time.Now(), io.Discard)
	close(stop)
	<-done
	if err != nil {
		t.Fatal(err)
	}
	if peak.Load() > 64<<20 {
		t.Fatalf("streaming heap exceeds bounded fixture ceiling: %d", peak.Load())
	}
	t.Logf("singleChainDataBytes>=%d streamingHeapPeak=%d streamingElapsed=%s", pages*8*scalarBytes, peak.Load(), time.Since(started).Round(time.Millisecond))
}
