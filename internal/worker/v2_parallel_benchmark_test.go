package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	definition "github.com/javleds/api-engine/internal/definition/domain"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	executiondb "github.com/javleds/api-engine/internal/execution/infrastructure/sqlite"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	dbplatform "github.com/javleds/api-engine/internal/platform/sqlite"
	security "github.com/javleds/api-engine/internal/security/infrastructure"
)

// This fixture separates controlled network latency from immediate cache hits.
// Unlike fakeFetcher, a cache hit does not reserve a physical HTTP attempt.
type parallelBenchmarkFetcher struct {
	latency time.Duration
	body    []byte
}

func (f *parallelBenchmarkFetcher) Fetch(ctx context.Context, request fetch.FetchRequest) (fetch.FetchResult, error) {
	if f.latency == 0 {
		return fetch.FetchResult{CacheHit: true, Response: fetch.RawResponse{StatusCode: 200, Body: f.body}}, nil
	}
	if request.BeforeAttempt != nil {
		if err := request.BeforeAttempt(ctx); err != nil {
			return fetch.FetchResult{}, err
		}
	}
	if request.OnAttempt != nil {
		request.OnAttempt()
	}
	timer := time.NewTimer(f.latency)
	defer timer.Stop()
	select {
	case <-timer.C:
		return fetch.FetchResult{Response: fetch.RawResponse{StatusCode: 200, Body: f.body, Attempts: 1}}, nil
	case <-ctx.Done():
		return fetch.FetchResult{}, ctx.Err()
	}
}

func BenchmarkV2ExecutionSchedulers(b *testing.B) {
	roots := make([]definition.CompiledCall, 8)
	for i := range roots {
		roots[i] = call(fmt.Sprintf("root%d", i), "single", fmt.Sprintf("/root%d", i))
	}
	compiledDefinition := compiled(roots...)
	body := []byte(`{"id":1,"name":"bulbasaur","number":9007199254740993,"stats":[{"base":45,"effort":0},{"base":49,"effort":0},{"base":65,"effort":1}],"types":["grass","poison"],"sprites":{"front":"image-a","back":"image-b"}}`)
	for _, mode := range []string{"cold", "cached"} {
		b.Run(mode, func(b *testing.B) {
			latency := time.Duration(0)
			if mode == "cold" {
				latency = 100 * time.Millisecond
			}
			for _, scheduler := range []string{"v1", "legacy", "workers1", "workers8"} {
				b.Run(scheduler, func(b *testing.B) {
					b.ReportAllocs()
					b.StopTimer()
					for iteration := 0; iteration < b.N; iteration++ {
						engine := &Engine{Fetcher: &parallelBenchmarkFetcher{latency: latency, body: body}, Options: Options{MaxBranchGoroutines: 8}}
						if scheduler == "v1" {
							b.StartTimer()
							result := engine.Execute(context.Background(), compiledDefinition, "owner", "benchmark")
							b.StopTimer()
							if result.Status != execution.Completed {
								b.Fatalf("v1 failed: %+v", result)
							}
							continue
						}
						checkpoint := 2
						if scheduler == "legacy" {
							checkpoint = 1
						}
						if scheduler == "workers1" {
							engine.Options.MaxBranchGoroutines = 1
						}
						runner, store, closeDB := benchmarkParallelRunner(b, engine, compiledDefinition, checkpoint)
						b.StartTimer()
						err := runner.Run(context.Background(), "benchmark")
						b.StopTimer()
						if err != nil {
							closeDB()
							b.Fatal(err)
						}
						_, current, err := store.Load(context.Background(), "benchmark")
						if err != nil || current.Status != execution.Completed {
							closeDB()
							b.Fatalf("scheduler failed: %+v %v", current, err)
						}
						if mode == "cached" && (current.Progress.CacheHits != 8 || current.Progress.HTTPAttempts != 0) {
							closeDB()
							b.Fatalf("cache fixture made HTTP attempts: %+v", current.Progress)
						}
						closeDB()
					}
				})
			}
		})
	}
}

func benchmarkParallelRunner(b *testing.B, engine *Engine, definition definition.CompiledDefinition, checkpoint int) (*V2Runner, *executiondb.ExecutionStore, func()) {
	b.Helper()
	ctx := context.Background()
	db, err := dbplatform.Open(ctx, filepath.Join(b.TempDir(), "engine.db"), false)
	if err != nil {
		b.Fatal(err)
	}
	closeDB := func() { db.Close() }
	if err = dbplatform.Migrate(ctx, db, "engine"); err != nil {
		closeDB()
		b.Fatal(err)
	}
	protector, err := security.NewAESGCMProtector("test", map[string][]byte{"test": bytes.Repeat([]byte{3}, 32)})
	if err != nil {
		closeDB()
		b.Fatal(err)
	}
	store := executiondb.NewExecutionStore(db, protector, 10)
	raw, _ := json.Marshal(definition)
	now := time.Now()
	_, _, err = store.Create(ctx, execution.ExecutionSnapshot{ID: "benchmark", OwnerID: "owner", APIVersion: 2, Definition: raw, Fingerprint: "benchmark", CreatedAt: now, DeadlineAt: now.Add(time.Minute), MetadataExpiresAt: now.Add(48 * time.Hour)})
	if err != nil {
		closeDB()
		b.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE v2_state SET checkpoint_version=? WHERE execution_id='benchmark'`, checkpoint); err != nil {
		closeDB()
		b.Fatal(err)
	}
	return &V2Runner{Store: store, V2Store: store, Finalizer: executiondb.NewExecutionFinalizer(store), Engine: engine}, store, closeDB
}
