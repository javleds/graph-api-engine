package ports

import (
	"context"

	"github.com/javleds/api-engine/internal/notification/domain"
)

// SourceEventOutbox locates a ready event without requiring completion-order projection.
type SourceEventOutbox interface {
	NextSourceEvent(context.Context, domain.NodeEventCursor) (int64, error)
}
