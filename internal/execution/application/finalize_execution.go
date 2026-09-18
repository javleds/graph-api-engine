package application

import (
	"context"
	"errors"
	"github.com/javleds/api-engine/internal/execution/application/ports"
	"github.com/javleds/api-engine/internal/execution/domain"
	"time"
)

// FinalizeExecution prepares the event once and retries only durable publication.
type FinalizeExecution struct {
	Store       ports.ExecutionStore
	Writer      ports.ExecutionFinalizer
	Prepare     func(context.Context, domain.ExecutionSnapshot, *domain.ExecutionCompletion) error
	OnFailure   func(error)
	OnPublished func(domain.ExecutionCompletion)
}

func (f *FinalizeExecution) Finish(ctx context.Context, completion domain.ExecutionCompletion) error {
	snapshot, existing, err := f.Store.Load(ctx, completion.ID)
	if err != nil {
		return err
	}
	if existing.Status.Terminal() {
		return domain.ErrVersionConflict
	}
	if completion.Report.Errors == nil {
		completion.Report.Errors = []domain.NodeFailure{}
	}
	if completion.Report.Roots == nil {
		completion.Report.Roots = []domain.RootOutcome{}
	}
	if f.Prepare != nil {
		if err := f.Prepare(ctx, snapshot, &completion); err != nil {
			return err
		}
	}
	for attempt := 0; attempt < 3; attempt++ {
		err = f.Writer.Finish(ctx, completion)
		if err == nil {
			if f.OnPublished != nil {
				f.OnPublished(completion)
			}
			return nil
		}
		if errors.Is(err, domain.ErrVersionConflict) {
			return err
		}
		if attempt == 2 {
			break
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	if f.OnFailure != nil {
		f.OnFailure(err)
	}
	return err
}
