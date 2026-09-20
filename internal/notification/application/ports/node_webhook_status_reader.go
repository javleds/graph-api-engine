package ports

import (
	"context"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/notification/domain"
)

type NodeWebhookStatusReader interface {
	NodeSummary(context.Context, string, execution.ExecutionID) (domain.NodeWebhookSummary, error)
	DeliveryPage(context.Context, string, execution.ExecutionID, string, int) (domain.WebhookDeliveryPage, error)
}
