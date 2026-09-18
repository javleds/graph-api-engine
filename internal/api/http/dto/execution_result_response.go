package dto

import (
	"encoding/json"
	"github.com/javleds/api-engine/internal/execution/domain"
)

type ExecutionResultResponse struct {
	ID           domain.ExecutionID       `json:"id"`
	Status       domain.ExecutionStatus   `json:"status"`
	Result       json.RawMessage          `json:"result"`
	Errors       []domain.NodeFailure     `json:"errors"`
	ErrorSummary domain.ErrorSummary      `json:"errorSummary"`
	Roots        []domain.RootOutcome     `json:"roots"`
	Error        *domain.ExecutionFailure `json:"error"`
}
