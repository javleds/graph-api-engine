package application

import (
	"context"
	"errors"
	"github.com/javleds/api-engine/internal/execution/domain"
	"testing"
	"time"
)

func TestReplayWinsWhenCompetingCreationCommitsDuringAdmission(t *testing.T) {
	for _, capacityCheck := range []bool{false, true} {
		store := &lifecycleStore{current: domain.Execution{ID: "winner", Status: domain.Running}}
		create := CreateExecution{Store: store, Dispatcher: rejectingDispatcher{}, IdentityKey: []byte("stable identity")}
		command := CreateExecutionCommand{OwnerID: "owner", IdempotencyKey: "same", Definition: []byte(`{"normalized":true}`)}
		store.current.Fingerprint = create.identity("request", string(command.Definition))
		if capacityCheck {
			create.CheckCapacity = func(context.Context) error { return domain.ErrEngineBusy }
		}
		result, err := create.Execute(context.Background(), command)
		if err != nil || !result.Replay || result.Execution.ID != "winner" {
			t.Fatalf("capacity=%v result=%+v err=%v", capacityCheck, result, err)
		}
	}
}

func TestRecoveryPreservesAvailableReportAndDoesNotReexecute(t *testing.T) {
	report := domain.EmptyReport()
	report.Errors = []domain.NodeFailure{{Code: "UPSTREAM_NOT_FOUND", Scope: "node"}}
	report.ErrorSummary = domain.ErrorSummary{Total: 1, Returned: 1}
	store := &lifecycleStore{snapshot: domain.ExecutionSnapshot{ID: "job"}, current: domain.Execution{ID: "job", Version: 1, Status: domain.Running, Report: report, Progress: domain.ExecutionProgress{HTTPAttempts: 3}}}
	calls := 0
	recovery := RecoverExecutions{Store: store, Finalizer: finalizerFunc(func(_ context.Context, c domain.ExecutionCompletion) error {
		calls++
		if c.Status != domain.Failed || c.Error.Code != "PROCESS_INTERRUPTED" || c.Report.ErrorSummary.Total != 1 || c.Progress.HTTPAttempts != 3 || len(c.Data) != 0 {
			t.Fatalf("completion %+v", c)
		}
		store.current.Status = domain.Failed
		return nil
	})}
	if err := recovery.Execute(context.Background(), []domain.ExecutionID{"job"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := recovery.Execute(context.Background(), []domain.ExecutionID{"job"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("recovery called %d times", calls)
	}
}

func TestFinalizationRetriesPreparedEventAndSignalsExhaustion(t *testing.T) {
	store := &lifecycleStore{snapshot: domain.ExecutionSnapshot{ID: "job"}, current: domain.Execution{ID: "job", Status: domain.Running}}
	prepared, calls := 0, 0
	finalizer := FinalizeExecution{Store: store, Prepare: func(_ context.Context, _ domain.ExecutionSnapshot, c *domain.ExecutionCompletion) error {
		prepared++
		c.Delivery = &domain.WebhookRecord{EventID: "stable"}
		return nil
	}, Writer: finalizerFunc(func(_ context.Context, c domain.ExecutionCompletion) error {
		calls++
		if c.Delivery.EventID != "stable" || c.Report.Errors == nil || c.Report.Roots == nil {
			t.Fatalf("completion %+v", c)
		}
		if calls < 3 {
			return errors.New("transient disk error")
		}
		return nil
	})}
	if err := finalizer.Finish(context.Background(), domain.ExecutionCompletion{ID: "job"}); err != nil {
		t.Fatal(err)
	}
	if prepared != 1 || calls != 3 {
		t.Fatalf("prepared=%d calls=%d", prepared, calls)
	}
	notified := false
	finalizer.OnFailure = func(error) { notified = true }
	finalizer.Writer = finalizerFunc(func(context.Context, domain.ExecutionCompletion) error { return errors.New("disk unavailable") })
	if err := finalizer.Finish(context.Background(), domain.ExecutionCompletion{ID: "job"}); err == nil || !notified {
		t.Fatalf("unreported failure %v", err)
	}
}
