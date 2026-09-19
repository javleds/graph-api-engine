package dto

import "github.com/javleds/api-engine/internal/execution/domain"

// CancelExecutionResponse reports the durable winner of cancel/complete races.
type CancelExecutionResponse struct {
	ID     domain.ExecutionID       `json:"id"`
	Status domain.ExecutionStatus   `json:"status"`
	Error  *domain.ExecutionFailure `json:"error"`
	Links  ExecutionLinks           `json:"links"`
}
