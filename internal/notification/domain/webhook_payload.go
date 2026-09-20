package domain

import (
	"encoding/json"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	"time"
)

type WebhookPayload struct {
	ResultCount         *int64                      `json:"resultCount,omitempty"`
	LastResultSequence  string                      `json:"lastResultSequence,omitempty"`
	LastSequence        string                      `json:"lastSequence,omitempty"`
	ResultExpiresAt     *time.Time                  `json:"resultExpiresAt,omitempty"`
	EventID             string                      `json:"eventId"`
	Type                string                      `json:"type"`
	ID                  execution.ExecutionID       `json:"id"`
	Status              execution.ExecutionStatus   `json:"status"`
	OccurredAt          time.Time                   `json:"occurredAt"`
	ResultIncluded      bool                        `json:"resultIncluded"`
	ErrorsIncluded      bool                        `json:"errorsIncluded"`
	Result              json.RawMessage             `json:"result,omitempty"`
	ResultOmittedReason string                      `json:"resultOmittedReason,omitempty"`
	ErrorsOmittedReason string                      `json:"errorsOmittedReason,omitempty"`
	Errors              []execution.NodeFailure     `json:"errors"`
	ErrorSummary        execution.ErrorSummary      `json:"errorSummary"`
	Roots               []execution.RootOutcome     `json:"roots"`
	Error               *execution.ExecutionFailure `json:"error"`
	Links               WebhookLinks                `json:"links"`
}
