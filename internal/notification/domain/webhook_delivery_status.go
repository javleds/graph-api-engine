package domain

import execution "github.com/javleds/api-engine/internal/execution/domain"

type WebhookDeliveryStatus struct {
	ID       string                      `json:"id"`
	EventID  string                      `json:"eventId"`
	NodeID   string                      `json:"nodeId"`
	Sequence string                      `json:"sequence"`
	Status   string                      `json:"status"`
	Attempts int                         `json:"attempts"`
	Error    *execution.ExecutionFailure `json:"error"`
}
