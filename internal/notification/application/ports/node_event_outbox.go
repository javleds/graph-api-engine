package ports

import (
	"context"
	"github.com/javleds/api-engine/internal/notification/domain"
)

type NodeEventOutbox interface {
	PendingNodeExecutions(context.Context, string, int) ([]domain.NodeEventCursor, error)
	SaveNodeDelivery(context.Context, domain.NodeEventCursor, domain.PreparedNodeDelivery, int, int64) (bool, error)
}
