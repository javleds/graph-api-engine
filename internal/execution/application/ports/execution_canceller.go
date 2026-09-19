package ports

import "github.com/javleds/api-engine/internal/execution/domain"

// ExecutionCanceller stops local work after its durable terminal state is confirmed.
type ExecutionCanceller interface {
	CancelExecution(domain.ExecutionID)
}
