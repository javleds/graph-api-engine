package ports

import (
	"context"
	executiondomain "github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/notification/domain"
	"time"
)

type WebhookOutbox interface {
	Claim(context.Context, string, time.Time, time.Duration) (*domain.WebhookDelivery, error)
	SaveAttempt(context.Context, domain.WebhookDelivery, bool, *time.Time, int, string, time.Time) error
	Recover(context.Context, string, time.Time) error
	Status(context.Context, string, executiondomain.ExecutionID) (*domain.WebhookStatus, error)
}
