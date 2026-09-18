package ports

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/domain"
	"time"
)

type ExecutionStore interface {
	Create(context.Context, domain.ExecutionSnapshot) (domain.Execution, bool, error)
	FindReplay(context.Context, string, string) (domain.Execution, error)
	Load(context.Context, domain.ExecutionID) (domain.ExecutionSnapshot, domain.Execution, error)
	Claim(context.Context, domain.ExecutionID, string, int64, time.Time) (bool, error)
}
