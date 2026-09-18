package ports

import (
	"context"
	execution "github.com/javleds/api-engine/internal/execution/domain"
)

// ProgressRecorder persists aggregate snapshots, never individual requests.
type ProgressRecorder interface {
	RecordProgress(context.Context, execution.ExecutionID, execution.ExecutionProgress) error
}
