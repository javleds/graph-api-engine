package dto

import "github.com/javleds/api-engine/internal/execution/domain"

type CreateExecutionResponse struct {
	ID     domain.ExecutionID     `json:"id"`
	Status domain.ExecutionStatus `json:"status"`
	Links  ExecutionLinks         `json:"links"`
}
