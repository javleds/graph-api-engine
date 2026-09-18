package application

import (
	"context"
	"errors"
	"github.com/javleds/api-engine/internal/execution/application/ports"
	"github.com/javleds/api-engine/internal/execution/domain"
	"time"
)

type RecoverExecutions struct {
	Store     ports.ExecutionStore
	Finalizer ports.ExecutionFinalizer
}

func (r RecoverExecutions) Execute(ctx context.Context, ids []domain.ExecutionID, now time.Time) error {
	for _, id := range ids {
		_, current, err := r.Store.Load(ctx, id)
		if err != nil {
			return err
		}
		if current.Status.Terminal() {
			continue
		}
		completion := domain.ExecutionCompletion{ID: id, ExpectedVersion: current.Version, Status: domain.Failed, Error: &domain.ExecutionFailure{Code: "PROCESS_INTERRUPTED", Message: "The previous process stopped before terminal publication."}, Report: current.Report, Progress: current.Progress, FinishedAt: now}
		if err := r.Finalizer.Finish(ctx, completion); err != nil && !errors.Is(err, domain.ErrVersionConflict) {
			return err
		}
	}
	return nil
}
