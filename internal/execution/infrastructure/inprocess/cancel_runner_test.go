package inprocess

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/domain"
)

type cancelRunner struct {
	started chan domain.ExecutionID
	stopped chan domain.ExecutionID
}

func (r *cancelRunner) Run(ctx context.Context, id domain.ExecutionID) error {
	r.started <- id
	<-ctx.Done()
	r.stopped <- id
	return ctx.Err()
}
