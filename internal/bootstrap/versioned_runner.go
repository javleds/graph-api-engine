package bootstrap

import (
	"context"
	"fmt"
	"github.com/javleds/api-engine/internal/execution/application/ports"
	"github.com/javleds/api-engine/internal/execution/domain"
)

// Dispatch by persisted protocol, never by a caller's current URL.
type versionedRunner struct {
	store   ports.ExecutionStore
	runners map[int]ports.ExecutionRunner
}

func (r versionedRunner) Run(ctx context.Context, id domain.ExecutionID) error {
	_, current, err := r.store.Load(ctx, id)
	if err != nil {
		return err
	}
	version := current.APIVersion
	if version == 0 {
		version = 1
	}
	runner, found := r.runners[version]
	if !found {
		return fmt.Errorf("execution %s has unsupported protocol version %d", id, version)
	}
	return runner.Run(ctx, id)
}
