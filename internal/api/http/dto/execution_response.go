package dto

import (
	"github.com/javleds/api-engine/internal/execution/domain"
	notification "github.com/javleds/api-engine/internal/notification/domain"
	"time"
)

type ExecutionResponse struct {
	NodeWebhookSummary *notification.NodeWebhookSummary `json:"nodeWebhookSummary,omitempty"`
	V2                 *domain.V2State                  `json:"v2,omitempty"`
	ID                 domain.ExecutionID               `json:"id"`
	Status             domain.ExecutionStatus           `json:"status"`
	CreatedAt          time.Time                        `json:"createdAt"`
	StartedAt          *time.Time                       `json:"startedAt"`
	FinishedAt         *time.Time                       `json:"finishedAt"`
	Progress           domain.ExecutionProgress         `json:"progress"`
	Webhook            *WebhookStatusResponse           `json:"webhook"`
	Error              *domain.ExecutionFailure         `json:"error"`
	Errors             []domain.NodeFailure             `json:"errors"`
	ErrorSummary       domain.ErrorSummary              `json:"errorSummary"`
	Roots              []domain.RootOutcome             `json:"roots"`
	Links              ExecutionLinks                   `json:"links"`
}
