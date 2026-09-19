package application

import (
	"context"
	"errors"
	"time"

	"github.com/javleds/api-engine/internal/execution/application/ports"
	"github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/platform/clock"
)

type CancelExecution struct {
	Store              ports.ExecutionStore
	Reader             ports.ExecutionReader
	Finalizer          ports.ExecutionFinalizer
	Canceller          ports.ExecutionCanceller
	Clock              clock.Clock
	PersistenceTimeout time.Duration
	ResultRetention    time.Duration
}

func (c *CancelExecution) Execute(ctx context.Context, owner string, id domain.ExecutionID, apiVersion int) (domain.Execution, error) {
	current, err := c.Reader.Read(ctx, owner, id)
	if err != nil {
		return domain.Execution{}, err
	}
	if current.APIVersion != apiVersion {
		return domain.Execution{}, domain.ErrNotFound
	}
	timeout := c.PersistenceTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	persist, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	for {
		if current.Status.Terminal() {
			c.stopCancelled(current)
			return current, nil
		}
		if err := persist.Err(); err != nil {
			return domain.Execution{}, err
		}
		completion, err := c.completion(persist, current)
		if err != nil {
			return domain.Execution{}, err
		}
		err = c.Finalizer.Finish(persist, completion)
		if err != nil && !errors.Is(err, domain.ErrVersionConflict) {
			return domain.Execution{}, err
		}
		if err == nil {
			// Do not depend on another storage read to deliver the stop signal after commit.
			c.Canceller.CancelExecution(id)
			current.Status = completion.Status
			current.Error = completion.Error
			current.Report = completion.Report
			current.Progress = completion.Progress
			current.FinishedAt = &completion.FinishedAt
			current.ResultExpiresAt = &completion.ResultExpiresAt
			current.Version++
			return current, nil
		}
		timer := time.NewTimer(5 * time.Millisecond)
		select {
		case <-persist.Done():
			timer.Stop()
			return domain.Execution{}, persist.Err()
		case <-timer.C:
		}
		current, err = c.Reader.Read(persist, owner, id)
		if err != nil {
			return domain.Execution{}, err
		}
		if current.APIVersion != apiVersion {
			return domain.Execution{}, domain.ErrNotFound
		}
	}
}

func (c *CancelExecution) stopCancelled(current domain.Execution) {
	if current.Error != nil && current.Error.Code == "EXECUTION_CANCELLED" {
		c.Canceller.CancelExecution(current.ID)
	}
}

func (c *CancelExecution) completion(ctx context.Context, current domain.Execution) (domain.ExecutionCompletion, error) {
	now := c.Clock.Now()
	retention := c.ResultRetention
	if retention <= 0 {
		retention = 7 * 24 * time.Hour
	}
	completion := domain.ExecutionCompletion{
		ID: current.ID, APIVersion: current.APIVersion, ExpectedVersion: current.Version,
		Status: domain.Failed, Error: &domain.ExecutionFailure{Code: "EXECUTION_CANCELLED", Message: "La ejecución fue cancelada por el usuario."},
		Report: current.Report, Progress: current.Progress, FinishedAt: now, ResultExpiresAt: now.Add(retention),
	}
	if current.APIVersion == 2 {
		if err := c.readCheckpoint(ctx, &completion); err != nil {
			return domain.ExecutionCompletion{}, err
		}
	}
	return completion, nil
}
