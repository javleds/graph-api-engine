package bootstrap

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/application/ports"
	"github.com/javleds/api-engine/internal/execution/domain"
)

// Dispatch by persisted protocol, never by a caller's current URL.
type versionedRunner struct {
	store ports.ExecutionStore
	v1    ports.ExecutionRunner
	v2    ports.ExecutionRunner
}

func (r versionedRunner) Run(ctx context.Context, id domain.ExecutionID) error {
	_, current, err := r.store.Load(ctx, id)
	if err != nil {
		return err
	}
	if current.APIVersion == 2 {
		return r.v2.Run(ctx, id)
	}
	return r.v1.Run(ctx, id)
}
