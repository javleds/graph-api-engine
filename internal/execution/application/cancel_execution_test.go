package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/platform/clock"
)

func TestCancelExecutionRequiresOwnerAndVersion(t *testing.T) {
	reader := &cancelReader{current: domain.Execution{ID: "job", OwnerID: "owner", APIVersion: 1, Status: domain.Running}}
	usecase := CancelExecution{Reader: reader}
	for _, candidate := range []cancelAccessCase{{"other", 1}, {"owner", 2}} {
		if _, err := usecase.Execute(context.Background(), candidate.owner, "job", candidate.version); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("got %v", err)
		}
	}
}

func TestCancelExecutionPersistsBeforeSignalAndSurvivesDisconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := &cancelReader{current: domain.Execution{ID: "job", OwnerID: "owner", APIVersion: 1, Status: domain.Pending}, afterRead: cancel}
	signal := &cancelSignal{}
	usecase := CancelExecution{Reader: reader, Canceller: signal, Clock: clock.RealClock{}}
	calls := 0
	usecase.Finalizer = finalizerFunc(func(ctx context.Context, c domain.ExecutionCompletion) error {
		calls++
		if ctx.Err() != nil || len(signal.ids) != 0 {
			t.Fatal("cancelled before persistence")
		}
		if c.Status != domain.Failed || c.Error.Code != "EXECUTION_CANCELLED" {
			t.Fatalf("bad completion %+v", c)
		}
		reader.current.Status = c.Status
		reader.current.Error = c.Error
		return nil
	})
	result, err := usecase.Execute(ctx, "owner", "job", 1)
	if err != nil || result.Status != domain.Failed || len(signal.ids) != 1 {
		t.Fatalf("result %+v err %v signal %v", result, err, signal.ids)
	}
	if _, err = usecase.Execute(context.Background(), "owner", "job", 1); err != nil || calls != 1 {
		t.Fatalf("repeated cancellation: %v calls %d", err, calls)
	}
}

func TestCancelExecutionNeverSignalsOnPersistenceFailure(t *testing.T) {
	reader := &cancelReader{current: domain.Execution{ID: "job", OwnerID: "owner", APIVersion: 1, Status: domain.Running}}
	signal := &cancelSignal{}
	failure := errors.New("disk full")
	usecase := CancelExecution{Reader: reader, Canceller: signal, Clock: clock.RealClock{}, Finalizer: finalizerFunc(func(context.Context, domain.ExecutionCompletion) error { return failure })}
	if _, err := usecase.Execute(context.Background(), "owner", "job", 1); !errors.Is(err, failure) || len(signal.ids) != 0 {
		t.Fatalf("err %v signal %v", err, signal.ids)
	}
}

func TestCancelExecutionPreservesNaturalCompletionAndRetriesClaim(t *testing.T) {
	for _, natural := range []bool{true, false} {
		reader := &cancelReader{current: domain.Execution{ID: "job", OwnerID: "owner", APIVersion: 1, Status: domain.Pending}}
		signal := &cancelSignal{}
		calls := 0
		usecase := CancelExecution{Reader: reader, Canceller: signal, Clock: clock.RealClock{}}
		usecase.Finalizer = finalizerFunc(func(_ context.Context, c domain.ExecutionCompletion) error {
			calls++
			if calls == 1 {
				reader.current.Version++
				reader.current.Status = domain.Running
				if natural {
					reader.current.Status = domain.Completed
				}
				return domain.ErrVersionConflict
			}
			if c.ExpectedVersion != 1 {
				t.Fatal("stale version")
			}
			return nil
		})
		result, err := usecase.Execute(context.Background(), "owner", "job", 1)
		if err != nil {
			t.Fatal(err)
		}
		if natural && (result.Status != domain.Completed || len(signal.ids) != 0 || calls != 1) {
			t.Fatal("overwrote natural completion")
		}
		if !natural && (result.Status != domain.Failed || len(signal.ids) != 1 || calls != 2) {
			t.Fatal("did not retry claim")
		}
	}
}

func TestCancelExecutionConflictRetriesAreBounded(t *testing.T) {
	reader := &cancelReader{current: domain.Execution{ID: "job", OwnerID: "owner", APIVersion: 1, Status: domain.Running}}
	signal := &cancelSignal{}
	usecase := CancelExecution{Reader: reader, Canceller: signal, Clock: clock.RealClock{}, PersistenceTimeout: time.Millisecond, Finalizer: finalizerFunc(func(context.Context, domain.ExecutionCompletion) error { return domain.ErrVersionConflict })}
	if _, err := usecase.Execute(context.Background(), "owner", "job", 1); !errors.Is(err, context.DeadlineExceeded) || len(signal.ids) != 0 {
		t.Fatalf("err %v signal %v", err, signal.ids)
	}
}

func TestCancelV2PreservesCommittedDiagnosticsAndPendingWithoutCheckpoint(t *testing.T) {
	for _, pending := range []bool{true, false} {
		reader := &cancelReader{current: domain.Execution{ID: "job", OwnerID: "owner", APIVersion: 2, Status: domain.Running}}
		store := &cancelV2Store{task: domain.V2Task{Revision: 7, Payload: []byte(`{"report":{"roots":[{"name":"items","status":"running","dataAvailable":true}]},"progress":{"completedRootItems":7}}`)}}
		if pending {
			reader.current.Status = domain.Pending
			store.taskErr = domain.ErrNotFound
		}
		signal := &cancelSignal{}
		usecase := CancelExecution{Reader: reader, Store: store, Canceller: signal, Clock: clock.RealClock{}}
		usecase.Finalizer = finalizerFunc(func(_ context.Context, c domain.ExecutionCompletion) error {
			expectedRevision := int64(7)
			if pending {
				expectedRevision = -1
			}
			if c.ExpectedCheckpointRevision == nil || *c.ExpectedCheckpointRevision != expectedRevision {
				t.Fatal("missing checkpoint revision guard")
			}
			if c.LastSequence != "13" {
				t.Fatalf("missing sequence guard: %q", c.LastSequence)
			}
			if c.Progress.LogicalRequests != 3 || c.Progress.HTTPAttempts != 4 {
				t.Fatal("lost durable budgets")
			}
			if !pending && (len(c.Report.Roots) != 1 || c.Report.Roots[0].Status != domain.CompletedWithErrors || c.Progress.CompletedRootItems != 7) {
				t.Fatalf("lost diagnostics %+v", c)
			}
			return nil
		})
		if _, err := usecase.Execute(context.Background(), "owner", "job", 2); err != nil {
			t.Fatal(err)
		}
	}
}
