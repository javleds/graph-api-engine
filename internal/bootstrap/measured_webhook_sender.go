package bootstrap

import (
	"context"
	"github.com/javleds/api-engine/internal/notification/application/ports"
	"github.com/javleds/api-engine/internal/notification/domain"
	"github.com/javleds/api-engine/internal/platform/observability"
)

type measuredWebhookSender struct {
	sender  ports.WebhookSender
	metrics *observability.Metrics
}

func (s measuredWebhookSender) Send(ctx context.Context, d domain.WebhookDelivery) (domain.WebhookAttempt, error) {
	s.metrics.Add("webhook_attempts_total", 1)
	attempt, err := s.sender.Send(ctx, d)
	if attempt.Accepted() {
		s.metrics.Add("webhook_accepted_total", 1)
	} else {
		s.metrics.Add("webhook_rejected_total", 1)
	}
	return attempt, err
}
