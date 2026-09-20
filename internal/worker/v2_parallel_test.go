package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	definition "github.com/javleds/api-engine/internal/definition/domain"
	executionports "github.com/javleds/api-engine/internal/execution/application/ports"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	executiondb "github.com/javleds/api-engine/internal/execution/infrastructure/sqlite"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	dbplatform "github.com/javleds/api-engine/internal/platform/sqlite"
	security "github.com/javleds/api-engine/internal/security/infrastructure"
)

type observedParallelStore struct {
	executionports.V2Store
	results chan execution.V2Result
}

func (s *observedParallelStore) CommitV2Task(ctx context.Context, commit execution.V2Commit) error {
	if err := s.V2Store.CommitV2Task(ctx, commit); err != nil {
		return err
	}
	for _, result := range commit.Results {
		s.results <- result
	}
	return nil
}

func parallelResultRunner(t *testing.T, fake *fakeFetcher, limit int, roots ...definition.CompiledCall) (*V2Runner, *executiondb.ExecutionStore, <-chan execution.V2Result) {
	t.Helper()
	ctx := context.Background()
	db, err := dbplatform.Open(ctx, filepath.Join(t.TempDir(), "engine.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = dbplatform.Migrate(ctx, db, "engine"); err != nil {
		t.Fatal(err)
	}
	protector, err := security.NewAESGCMProtector("test", map[string][]byte{"test": bytes.Repeat([]byte{3}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	store := executiondb.NewExecutionStore(db, protector, 10)
	raw, _ := json.Marshal(compiled(roots...))
	now := time.Now()
	_, _, err = store.Create(ctx, execution.ExecutionSnapshot{ID: "parallel", OwnerID: "owner", APIVersion: 2, Definition: raw, Fingerprint: "parallel", CreatedAt: now, DeadlineAt: now.Add(time.Minute), MetadataExpiresAt: now.Add(48 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.ReadV2State(ctx, "parallel")
	if err != nil || state.CheckpointVersion != 2 {
		t.Fatalf("new execution must use concurrent checkpoint: %+v %v", state, err)
	}
	observed := &observedParallelStore{V2Store: store, results: make(chan execution.V2Result, 100)}
	runner := &V2Runner{Store: store, V2Store: observed, Finalizer: executiondb.NewExecutionFinalizer(store), Engine: &Engine{Fetcher: fake, Options: Options{MaxBranchGoroutines: limit}}}
	return runner, store, observed.results
}

func runParallelTest(t *testing.T, runner *V2Runner) (context.Context, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	done := make(chan error, 1)
	finished := make(chan struct{})
	go func() { defer close(finished); done <- runner.Run(ctx, "parallel") }()
	t.Cleanup(func() { cancel(); <-finished })
	return ctx, done
}

func awaitParallelResult(t *testing.T, ctx context.Context, results <-chan execution.V2Result) execution.V2Result {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-ctx.Done():
		t.Fatal("no completed chain before deadline")
		return execution.V2Result{}
	}
}

func awaitParallelRun(t *testing.T, ctx context.Context, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("concurrent execution did not terminate")
	}
}

func TestV2ParallelRootsAndItemsPublishCompleteFastChainFirst(t *testing.T) {
	release := make(chan struct{})
	started := make(chan string, 4)
	fake := &fakeFetcher{handle: func(ctx context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		switch {
		case strings.HasSuffix(r.Request.URL, "/pokemon"):
			return response(`[{"id":1},{"id":2}]`)
		case strings.HasSuffix(r.Request.URL, "/species/2"):
			return response(`{"number":9007199254740993}`)
		case strings.HasSuffix(r.Request.URL, "/language"), strings.HasSuffix(r.Request.URL, "/species/1"):
			started <- r.Request.URL
			select {
			case <-release:
				return response(`{"slow":true}`)
			case <-ctx.Done():
				return fetch.FetchResult{}, ctx.Err()
			}
		default:
			return fetch.FetchResult{}, fmt.Errorf("unexpected URL %s", r.Request.URL)
		}
	}}
	runner, store, events := parallelResultRunner(t, fake, 4, call("language", "single", "/language"), call("pokemon", "collection", "/pokemon", call("species", "single", "/species/{parent.id}")))
	ctx, done := runParallelTest(t, runner)
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("independent slow branches were not admitted")
		}
	}
	fast := awaitParallelResult(t, ctx, events)
	if fast.Root != "pokemon" || fast.Index == nil || *fast.Index != 1 {
		t.Fatalf("wrong first result: %+v", fast)
	}
	page, err := store.ReadV2Results(ctx, "owner", "parallel", 0, 0, 100, time.Now())
	if err != nil || len(page.Results) != 1 || page.Terminal {
		t.Fatalf("unexpected publication before slow branches finish: %+v %v", page, err)
	}
	var body bytes.Buffer
	if err = store.WriteV2Result(ctx, "owner", "parallel", page.Results[0].NodeID, time.Now(), &body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body.String(), `"data":{"id":2,"species":{"number":9007199254740993}}`) {
		t.Fatalf("chain incomplete: %s", body.String())
	}
	close(release)
	awaitParallelRun(t, ctx, done)
	page, err = store.ReadV2Results(ctx, "owner", "parallel", 0, 0, 100, time.Now())
	if err != nil || len(page.Results) != 3 || !page.Terminal {
		t.Fatalf("missing final results: %+v %v", page, err)
	}
}

func TestV2ParallelIndependentChildrenRespectWorkerLimit(t *testing.T) {
	for _, limit := range []int{1, 4} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			permits := make(chan struct{})
			started := make(chan struct{}, 8)
			var active, peak atomic.Int32
			fake := &fakeFetcher{handle: func(ctx context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
				if strings.HasSuffix(r.Request.URL, "/root") {
					return response(`{}`)
				}
				n := active.Add(1)
				defer active.Add(-1)
				for previous := peak.Load(); n > previous; previous = peak.Load() {
					if peak.CompareAndSwap(previous, n) {
						break
					}
				}
				started <- struct{}{}
				select {
				case <-permits:
					return response(`{"ok":true}`)
				case <-ctx.Done():
					return fetch.FetchResult{}, ctx.Err()
				}
			}}
			children := make([]definition.CompiledCall, 8)
			for i := range children {
				children[i] = call(fmt.Sprintf("child%d", i), "single", fmt.Sprintf("/child%d", i))
			}
			runner, store, events := parallelResultRunner(t, fake, limit, call("root", "single", "/root", children...))
			ctx, done := runParallelTest(t, runner)
			for offset := 0; offset < len(children); offset += limit {
				for i := 0; i < limit; i++ {
					select {
					case <-started:
					case <-ctx.Done():
						t.Fatal("children failed to start up to configured concurrency")
					}
				}
				select {
				case premature := <-events:
					t.Fatalf("single result published with children still pending: %+v", premature)
				default:
				}
				for i := 0; i < limit; i++ {
					select {
					case permits <- struct{}{}:
					case <-ctx.Done():
						t.Fatal("children failed to accept completion")
					}
				}
			}
			awaitParallelRun(t, ctx, done)
			if peak.Load() != int32(limit) {
				t.Fatalf("peak=%d configured=%d", peak.Load(), limit)
			}
			page, err := store.ReadV2Results(ctx, "owner", "parallel", 0, 0, 100, time.Now())
			if err != nil || len(page.Results) != 1 || page.Results[0].Status != "completed" {
				t.Fatalf("invalid single result: %+v %v", page, err)
			}
			var body bytes.Buffer
			if err = store.WriteV2Result(ctx, "owner", "parallel", page.Results[0].NodeID, time.Now(), &body); err != nil {
				t.Fatal(err)
			}
			for i := range children {
				if !strings.Contains(body.String(), fmt.Sprintf(`"child%d":{"ok":true}`, i)) {
					t.Fatalf("missing child %d: %s", i, body.String())
				}
			}
		})
	}
}

func TestV2ParallelFailuresRemainIsolatedToTheirCompletedChain(t *testing.T) {
	fake := &fakeFetcher{handle: func(ctx context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		if strings.HasSuffix(r.Request.URL, "/pokemon") {
			return response(`[{"id":1},{"id":2}]`)
		}
		if strings.HasSuffix(r.Request.URL, "/species/1") {
			return fetch.FetchResult{}, &fetch.UpstreamFailure{Code: "UPSTREAM_HTTP_ERROR"}
		}
		return response(`{"ok":true}`)
	}}
	runner, store, _ := parallelResultRunner(t, fake, 4, call("pokemon", "collection", "/pokemon", call("species", "single", "/species/{parent.id}")))
	ctx, done := runParallelTest(t, runner)
	awaitParallelRun(t, ctx, done)
	page, err := store.ReadV2Results(ctx, "owner", "parallel", 0, 0, 100, time.Now())
	if err != nil || len(page.Results) != 2 {
		t.Fatalf("results: %+v %v", page, err)
	}
	for _, result := range page.Results {
		if result.Index == nil {
			t.Fatal("collection result lost original index")
		}
		var body bytes.Buffer
		if err = store.WriteV2Result(ctx, "owner", "parallel", result.NodeID, time.Now(), &body); err != nil {
			t.Fatal(err)
		}
		if *result.Index == 0 {
			if result.Status != "completed_with_errors" || result.ErrorSummary.Total != 1 || !strings.Contains(body.String(), `"species":null`) || !strings.Contains(body.String(), "UPSTREAM_HTTP_ERROR") {
				t.Fatalf("failure missing from its chain: %+v %s", result, body.String())
			}
		} else if result.Status != "completed" || result.ErrorSummary.Total != 0 || strings.Contains(body.String(), "UPSTREAM_HTTP_ERROR") {
			t.Fatalf("failure leaked to independent chain: %+v %s", result, body.String())
		}
	}
}
