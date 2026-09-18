package ports

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/domain"
)

type ExecutionFinalizer interface {
	Finish(context.Context, domain.ExecutionCompletion) error
}
