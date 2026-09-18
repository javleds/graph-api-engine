package worker

import (
	"encoding/json"
	execution "github.com/javleds/api-engine/internal/execution/domain"
)

type resultEnvelope struct {
	ID           execution.ExecutionID       `json:"id"`
	Status       execution.ExecutionStatus   `json:"status"`
	Result       json.RawMessage             `json:"result"`
	Errors       []execution.NodeFailure     `json:"errors"`
	ErrorSummary execution.ErrorSummary      `json:"errorSummary"`
	Roots        []execution.RootOutcome     `json:"roots"`
	Error        *execution.ExecutionFailure `json:"error"`
}
