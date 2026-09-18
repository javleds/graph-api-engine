package ports

import (
	"context"
	"github.com/javleds/api-engine/internal/notification/domain"
)

type WebhookSender interface {
	Send(context.Context, domain.WebhookDelivery) (domain.WebhookAttempt, error)
}
