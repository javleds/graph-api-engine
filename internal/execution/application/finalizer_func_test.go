package application

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/domain"
)

type finalizerFunc func(context.Context, domain.ExecutionCompletion) error

func (f finalizerFunc) Finish(ctx context.Context, c domain.ExecutionCompletion) error {
	return f(ctx, c)
}
