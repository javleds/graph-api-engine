package ports

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/domain"
)

type ExecutionReader interface {
	Read(context.Context, string, domain.ExecutionID) (domain.Execution, error)
}
