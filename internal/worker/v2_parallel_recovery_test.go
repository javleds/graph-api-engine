package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	definition "github.com/javleds/api-engine/internal/definition/domain"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
)

func TestV2ParallelRecoveryPreservesConfirmedInputsAndEvents(t *testing.T) {
	var resumed atomic.Bool
	started := make(chan string, 2)
	fake := &fakeFetcher{handle: func(ctx context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		if !resumed.Load() && (strings.HasSuffix(r.Request.URL, "/slow") || strings.HasSuffix(r.Request.URL, "/grandchild")) {
			started <- r.Request.URL
			<-ctx.Done()
			return fetch.FetchResult{}, ctx.Err()
		}
		return response(`{"id":1}`)
	}}
	runner, store, events := parallelResultRunner(t, fake, 4, call("done", "single", "/done"), call("root", "single", "/root", call("fast", "single", "/fast", call("grandchild", "single", "/grandchild")), call("slow", "single", "/slow")))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	finished := make(chan struct{})
	go func() { defer close(finished); done <- runner.Run(ctx, "parallel") }()
	defer func() { cancel(); <-finished }()
	completed := awaitParallelResult(t, ctx, events)
	if completed.Root != "done" {
		t.Fatalf("unexpected complete result: %+v", completed)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("independent descendants were not admitted")
		}
	}
	before, err := store.ReadV2Results(ctx, "owner", "parallel", 0, 0, 100, time.Now())
	if err != nil || len(before.Results) != 1 {
		t.Fatalf("result before interruption: %+v %v", before, err)
	}
	cancel()
	select {
	case err = <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected resumable cancellation, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("workers did not stop after cancellation")
	}
	_, current, err := store.Load(context.Background(), "parallel")
	if err != nil || current.Status != execution.Running {
		t.Fatalf("cancellation finalized the execution: %+v %v", current, err)
	}
	resumed.Store(true)
	if err = store.RecoverV2(context.Background()); err != nil {
		t.Fatal(err)
	}
	resumeCtx, resumeCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer resumeCancel()
	if err = runner.Run(resumeCtx, "parallel"); err != nil {
		t.Fatal(err)
	}
	after, err := store.ReadV2Results(resumeCtx, "owner", "parallel", 0, 0, 100, time.Now())
	if err != nil || len(after.Results) != 2 || !after.Terminal {
		t.Fatalf("recovered results: %+v %v", after, err)
	}
	if after.Results[0].EventID != before.Results[0].EventID {
		t.Fatal("confirmed event identity changed on recovery")
	}
	for _, path := range []string{"done", "root", "fast"} {
		if count := fake.count("https://provider.test/" + path); count != 1 {
			t.Fatalf("confirmed input %s refetched %d times", path, count)
		}
	}
	for _, path := range []string{"slow", "grandchild"} {
		if count := fake.count("https://provider.test/" + path); count != 2 {
			t.Fatalf("interrupted request %s was not retried: %d", path, count)
		}
	}
	state, err := store.ReadV2State(resumeCtx, "parallel")
	if err != nil || state.Generation != 2 || state.Recoveries != 1 || state.LogicalRequests != 5 || state.HTTPAttempts != 7 {
		t.Fatalf("recovery budgets: %+v %v", state, err)
	}
}

func TestV2ParallelGlobalRequestBudgetsCannotBeOversubscribed(t *testing.T) {
	for _, kind := range []string{"logical", "attempt"} {
		for round := 0; round < 5; round++ {
			t.Run(fmt.Sprintf("%s/%d", kind, round), func(t *testing.T) {
				var calls atomic.Int32
				fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) {
					calls.Add(1)
					return response(`{"ok":true}`)
				}}
				roots := make([]definition.CompiledCall, 8)
				for i := range roots {
					roots[i] = call(fmt.Sprintf("root%d", i), "single", fmt.Sprintf("/root%d", i))
				}
				runner, store, _ := parallelResultRunner(t, fake, 8, roots...)
				if kind == "logical" {
					runner.Engine.Options.MaxLogicalRequests = 3
				} else {
					runner.Engine.Options.MaxHTTPAttempts = 3
				}
				ctx, done := runParallelTest(t, runner)
				awaitParallelRun(t, ctx, done)
				_, current, err := store.Load(ctx, "parallel")
				if err != nil || current.Status != execution.Failed || current.Error == nil || current.Error.Code != "EXECUTION_BUDGET_EXCEEDED" {
					t.Fatalf("budget failure not terminal: %+v %v", current, err)
				}
				state, err := store.ReadV2State(ctx, "parallel")
				if err != nil {
					t.Fatal(err)
				}
				counter := state.LogicalRequests
				if kind == "attempt" {
					counter = state.HTTPAttempts
				}
				if counter > 3 || calls.Load() > 3 || state.HTTPAttempts > 3 {
					t.Fatalf("oversubscribed global %s budget: %+v calls=%d", kind, state, calls.Load())
				}
			})
		}
	}
}
