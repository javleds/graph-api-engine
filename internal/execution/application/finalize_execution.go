package application

import (
	"context"
	"errors"
	"github.com/javleds/api-engine/internal/execution/application/ports"
	"github.com/javleds/api-engine/internal/execution/domain"
	"strconv"
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
	completion.APIVersion = existing.APIVersion
	if existing.APIVersion == 2 {
		store, ok := f.Store.(ports.V2Store)
		if !ok {
			return errors.New("v2 store is required")
		}
		state, err := store.ReadV2State(ctx, completion.ID)
		if err != nil {
			return err
		}
		sequence, err := strconv.ParseInt(state.LastSequence, 10, 64)
		if err != nil {
			return err
		}
		nextSequence := strconv.FormatInt(sequence+1, 10)
		// A cancellation supplies the sequence observed before reading its checkpoint.
		// Never attach stale diagnostics to a newer committed prefix.
		if completion.LastSequence != "" && completion.LastSequence != nextSequence {
			return domain.ErrVersionConflict
		}
		completion.LastSequence = nextSequence
		if completion.ResultExpiresAt.IsZero() {
			completion.ResultExpiresAt = completion.FinishedAt.Add(7 * 24 * time.Hour)
		}
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
