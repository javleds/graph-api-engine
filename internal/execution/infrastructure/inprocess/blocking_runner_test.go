package inprocess

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/domain"
)

type blockingRunner struct{ started, release chan struct{} }

func (r *blockingRunner) Run(ctx context.Context, _ domain.ExecutionID) error {
	close(r.started)
	select {
	case <-r.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
