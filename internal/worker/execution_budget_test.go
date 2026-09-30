package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	definition "github.com/javleds/api-engine/internal/definition/domain"
)

func TestRequestUsesNodeCachePolicy(t *testing.T) {
	root := call("root", "single", "/root")
	root.Cache = definition.CachePolicy{Enabled: true, TTL: time.Minute, IgnoreServerCachePolicy: true}
	graph := compiled(root)
	processor := &CallProcessor{engine: &Engine{}, definition: graph, budget: &ExecutionBudget{}, progress: &progressCounters{}}
	request, err := processor.buildRequest(context.Background(), root, NodeContext{})
	if err != nil {
		t.Fatal(err)
	}
	if !request.Policy.CacheEnabled || request.Policy.CacheTTL != time.Minute || !request.Policy.IgnoreServerCachePolicy {
		t.Fatalf("cache policy lost during request building: %+v", request.Policy)
	}
}

func TestCanceledSubscriberCannotSpendItsExecutionBudget(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	budget := &ExecutionBudget{maxAttempts: 2, cancel: cancel}
	definition := compiled(call("root", "single", "/root"))
	processor := &CallProcessor{engine: &Engine{}, definition: definition, budget: budget, progress: &progressCounters{}}
	request, err := processor.buildRequest(ctx, definition.Roots[0], NodeContext{})
	if err != nil {
		t.Fatal(err)
	}
	cancel(context.Canceled)
	// Shared physical work has a live service context after this subscriber leaves.
	if err = request.BeforeAttempt(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if budget.attempts.Load() != 0 || processor.progress.attempts.Load() != 0 {
		t.Fatal("canceled subscriber budget changed")
	}
}
func TestConcurrentAttemptReservationsNeverExceedExecutionBudget(t *testing.T) {
	_, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	budget := &ExecutionBudget{maxAttempts: 3, cancel: cancel}
	var successes atomic.Int32
	var group sync.WaitGroup
	for index := 0; index < 30; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			err := budget.attempt(context.Background())
			if err == nil {
				successes.Add(1)
			} else if !errors.Is(err, ErrBudgetExceeded) {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	if budget.attempts.Load() != 3 || successes.Load() != 3 {
		t.Fatal(budget.attempts.Load(), successes.Load())
	}
}
