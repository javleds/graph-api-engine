package worker

import (
	"context"
	"testing"

	execution "github.com/javleds/api-engine/internal/execution/domain"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
)

func TestV2ParallelDeadlineExpiredBeforeRunPublishesTerminal(t *testing.T) {
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) {
		t.Error("expired execution must not perform upstream HTTP")
		return response(`{}`)
	}}
	runner, store, _ := parallelResultRunner(t, fake, 2, call("root", "single", "/root"))
	runner.V2Store = expiredParallelStore{V2Store: runner.V2Store}
	if err := runner.Run(context.Background(), "parallel"); err != nil {
		t.Fatalf("expired job returned without durable finalization: %v", err)
	}
	_, result, err := store.Load(context.Background(), "parallel")
	if err != nil || result.Status != execution.Failed || result.Error == nil || result.Error.Code != "EXECUTION_TIMEOUT" {
		t.Fatalf("expected timeout terminal, got %+v: %v", result, err)
	}
	state, err := store.ReadV2State(context.Background(), "parallel")
	if err != nil || !state.Terminal || state.ResultCount != 0 || state.HTTPAttempts != 0 {
		t.Fatalf("invalid expired execution state: %+v %v", state, err)
	}
}
