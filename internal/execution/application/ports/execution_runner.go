package ports

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/domain"
)

type ExecutionRunner interface {
	Run(context.Context, domain.ExecutionID) error
}
