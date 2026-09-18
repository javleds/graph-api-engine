package application

import (
	"context"
	notification "github.com/javleds/api-engine/internal/notification/domain"
)

type fakeSender func(context.Context, notification.WebhookDelivery) (notification.WebhookAttempt, error)

func (f fakeSender) Send(ctx context.Context, d notification.WebhookDelivery) (notification.WebhookAttempt, error) {
	return f(ctx, d)
}
