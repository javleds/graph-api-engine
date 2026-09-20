package worker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	executionapp "github.com/javleds/api-engine/internal/execution/application"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/execution/infrastructure/inprocess"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	"github.com/javleds/api-engine/internal/platform/clock"
)

func TestV2ParallelCancellationPreservesCommittedActiveRootDiagnostics(t *testing.T) {
	blocked := make(chan struct{})
	stopped := make(chan struct{})
	fake := &fakeFetcher{handle: func(ctx context.Context, request fetch.FetchRequest) (fetch.FetchResult, error) {
		switch {
		case strings.HasSuffix(request.Request.URL, "/root"):
			return response(`{"id":1}`)
		case strings.HasSuffix(request.Request.URL, "/failure"):
			return fetch.FetchResult{}, &fetch.UpstreamFailure{Code: "UPSTREAM_HTTP_ERROR"}
		default:
			close(blocked)
			<-ctx.Done()
			close(stopped)
			return fetch.FetchResult{}, ctx.Err()
		}
	}}
	runner, store, _ := parallelResultRunner(t, fake, 4, call("root", "single", "/root", call("failure", "single", "/failure"), call("waiting", "single", "/waiting")))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dispatcher := inprocess.NewLocalExecutionDispatcher(ctx, runner, 1, 0)
	defer func() {
		dispatcher.Cancel()
		wait, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		if err := dispatcher.Wait(wait); err != nil {
			t.Error(err)
		}
	}()
	reservation, err := dispatcher.Reserve(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = reservation.Commit(ctx, "parallel"); err != nil {
		reservation.Cancel()
		t.Fatal(err)
	}
	select {
	case <-blocked:
	case <-ctx.Done():
		t.Fatal("independent waiting child never started")
	}
	// Observe a committed failure in an active task, independently of the summary
	// consumed by cancellation. The root cannot complete while its sibling waits.
	for {
		task, err := store.ReadV2Task(ctx, "parallel", "coordinator")
		if err != nil {
			t.Fatal(err)
		}
		var checkpoint v2ParallelCheckpoint
		if err = json.Unmarshal(task.Payload, &checkpoint); err != nil {
			t.Fatal(err)
		}
		var failures uint64
		for _, active := range checkpoint.Tasks {
			failures += active.Checkpoint.Report.ErrorSummary.Total
		}
		if failures == 1 {
			break
		}
		select {
		case <-time.After(5 * time.Millisecond):
		case <-ctx.Done():
			t.Fatal("child failure was never durably committed")
		}
	}
	usecase := executionapp.CancelExecution{Store: store, Reader: store, Finalizer: runner.Finalizer, Canceller: dispatcher, Clock: clock.RealClock{}}
	result, err := usecase.Execute(ctx, "owner", "parallel", 2)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != execution.Failed || result.Error == nil || result.Error.Code != "EXECUTION_CANCELLED" {
		t.Fatalf("invalid cancellation: %+v", result)
	}
	if result.Report.ErrorSummary.Total != 1 || len(result.Report.Errors) != 1 || result.Report.Errors[0].Code != "UPSTREAM_HTTP_ERROR" {
		t.Fatalf("committed diagnostic lost during cancellation: %+v", result.Report)
	}
	if len(result.Report.Roots) != 1 || !result.Report.Roots[0].DataAvailable || result.Report.Roots[0].Status != execution.CompletedWithErrors {
		t.Fatalf("active root state lost during cancellation: %+v", result.Report.Roots)
	}
	select {
	case <-stopped:
	case <-ctx.Done():
		t.Fatal("durable cancellation did not stop pending fetch")
	}
	if err = dispatcher.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	_, persisted, err := store.Load(ctx, "parallel")
	if err != nil || persisted.Report.ErrorSummary.Total != 1 || persisted.Report.Roots[0].Status != execution.CompletedWithErrors {
		t.Fatalf("worker overwrote cancelled diagnostics: %+v %v", persisted, err)
	}
	state, err := store.ReadV2State(ctx, "parallel")
	if err != nil || !state.Terminal || state.ResultCount != 0 {
		t.Fatalf("unfinished chain was published by cancellation: %+v %v", state, err)
	}
}
