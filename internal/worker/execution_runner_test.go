package worker

import (
	"context"
	"encoding/json"
	"errors"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	"testing"
	"time"
)

func TestRunnerFinalizesWithIndependentContext(t *testing.T) {
	raw, _ := json.Marshal(compiled(call("root", "single", "/root")))
	store := &memoryExecutionStore{snapshot: execution.ExecutionSnapshot{ID: "id", OwnerID: "owner", Definition: raw}, current: execution.Execution{ID: "id", Status: execution.Pending, Version: 1}}
	fake := &fakeFetcher{handle: func(ctx context.Context, _ fetch.FetchRequest) (fetch.FetchResult, error) {
		<-ctx.Done()
		return fetch.FetchResult{}, ctx.Err()
	}}
	runner := ExecutionRunner{Store: store, Finalizer: store, Engine: &Engine{Fetcher: fake, Options: Options{Timeout: time.Millisecond}}}
	if err := runner.Run(context.Background(), "id"); err != nil {
		t.Fatal(err)
	}
	if !store.finished || store.finishContextError != nil || store.completion.ExpectedVersion != 2 || store.completion.Error.Code != "EXECUTION_TIMEOUT" {
		t.Fatal(store)
	}
}
func TestRunnerStorageFailurePropagatesWithoutClaimingDurableOutcome(t *testing.T) {
	raw, _ := json.Marshal(compiled(call("root", "single", "/root")))
	storageError := errors.New("storage failed")
	store := &memoryExecutionStore{snapshot: execution.ExecutionSnapshot{ID: "id", Definition: raw}, current: execution.Execution{ID: "id", Status: execution.Pending, Version: 1}, failFinish: storageError}
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) { return response(`{}`) }}
	runner := ExecutionRunner{Store: store, Finalizer: store, Engine: &Engine{Fetcher: fake}}
	if err := runner.Run(context.Background(), "id"); !errors.Is(err, storageError) {
		t.Fatal(err)
	}
}
func TestRunnerNeverReplaysTerminalExecution(t *testing.T) {
	store := &memoryExecutionStore{current: execution.Execution{ID: "id", Status: execution.CompletedWithErrors, Version: 3}}
	runner := ExecutionRunner{Store: store, Finalizer: store}
	if err := runner.Run(context.Background(), "id"); err != nil || store.finished {
		t.Fatal(err, store)
	}
}
func TestRunnerPrepareCompletionHappensBeforeFinish(t *testing.T) {
	raw, _ := json.Marshal(compiled(call("root", "single", "/root")))
	store := &memoryExecutionStore{snapshot: execution.ExecutionSnapshot{ID: "id", Definition: raw}, current: execution.Execution{ID: "id", Status: execution.Pending, Version: 1}}
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) { return response(`{}`) }}
	prepared := false
	runner := ExecutionRunner{Store: store, Finalizer: store, Engine: &Engine{Fetcher: fake}, PrepareCompletion: func(ctx context.Context, _ execution.ExecutionSnapshot, completion *execution.ExecutionCompletion) error {
		if store.finished || ctx.Err() != nil || len(completion.Data) == 0 {
			t.Fatal("completion preparation lifecycle")
		}
		prepared = true
		return nil
	}}
	if err := runner.Run(context.Background(), "id"); err != nil || !prepared || !store.finished {
		t.Fatal(err)
	}
}
