package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBoundedFanoutLoadKeepsStatusResponsive(t *testing.T) {
	var active, peak, physical atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		physical.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/items" {
			items := make([]map[string]int, 100)
			for i := range items {
				items[i] = map[string]int{"id": i}
			}
			json.NewEncoder(w).Encode(items)
			return
		}
		current := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); current > old; old = peak.Load() {
			if peak.CompareAndSwap(old, current) {
				break
			}
		}
		once.Do(func() { close(started) })
		<-release
		time.Sleep(time.Millisecond)
		io.WriteString(w, `{"ok":true}`)
	}))
	defer upstream.Close()
	cfg := fixtureConfig(t, upstream.URL, upstream.URL)
	cfg.Fetch.MaxConcurrentHTTPRequests = 4
	cfg.Executions.MaxBranchGoroutines = 8
	fixture := openFixture(t, cfg)
	beforeGoroutines := runtime.NumGoroutine()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	id := fixture.create(t, definitionBody(upstream.URL, nil), "")
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("upstream did not start")
	}
	statusStarted := time.Now()
	status, data := fixture.request(t, "GET", "/v1/executions/"+id, fixture.token, "", nil)
	latency := time.Since(statusStarted)
	close(release)
	if status != 200 {
		t.Fatalf("status during work %d %s", status, data)
	}
	var state map[string]any
	json.Unmarshal(data, &state)
	if state["status"] != "running" {
		t.Fatal("status query did not run concurrently with blocked upstream")
	}
	terminal := fixture.terminal(t, id)
	if terminal["status"] != "completed" || peak.Load() > 4 || physical.Load() != 101 {
		t.Fatalf("limits/status physical=%d peak=%d result=%#v", physical.Load(), peak.Load(), terminal)
	}
	second := fixture.create(t, definitionBody(upstream.URL, nil), "")
	fixture.terminal(t, second)
	if physical.Load() != 101 {
		t.Fatal("hot execution ignored cache")
	}
	runtime.ReadMemStats(&after)
	walBytes := int64(0)
	for _, path := range []string{cfg.Storage.Path + "-wal", cfg.Cache.Path + "-wal"} {
		if info, err := os.Stat(path); err == nil {
			walBytes += info.Size()
		}
	}
	t.Log("fanout=100 coldAttempts=101 hotAttempts=0 peakHTTP="+strconv.Itoa(int(peak.Load())), "statusLatency", latency, "heapBefore", before.HeapAlloc, "heapAfter", after.HeapAlloc, "goroutinesBefore", beforeGoroutines, "goroutinesAfter", runtime.NumGoroutine(), "walBytes", walBytes)
}
