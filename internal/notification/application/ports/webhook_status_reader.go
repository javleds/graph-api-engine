package ports

import (
	"context"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/notification/domain"
)

type WebhookStatusReader interface {
	Status(context.Context, string, execution.ExecutionID) (*domain.WebhookStatus, error)
}
