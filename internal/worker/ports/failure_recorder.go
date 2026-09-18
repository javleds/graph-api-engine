package ports

import execution "github.com/javleds/api-engine/internal/execution/domain"

type FailureRecorder interface {
	Record(execution.NodeFailure)
	Snapshot() execution.ExecutionErrorReport
}
