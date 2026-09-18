package ports

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/domain"
	"time"
)

type ResultReader interface {
	ReadResult(context.Context, string, domain.ExecutionID, time.Time) (domain.Execution, []byte, error)
}
