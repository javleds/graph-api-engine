package application

import (
	"context"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/notification/domain"
)

type nodeOutbox struct {
	after int64
	saved []domain.PreparedNodeDelivery
	full  bool
}

func (o *nodeOutbox) PendingNodeExecutions(context.Context, string, int) ([]domain.NodeEventCursor, error) {
	return []domain.NodeEventCursor{{ExecutionID: "job", OwnerID: "owner", After: o.after}}, nil
}
func (o *nodeOutbox) SaveNodeDelivery(_ context.Context, c domain.NodeEventCursor, d domain.PreparedNodeDelivery, _ int, _ int64) (bool, error) {
	if o.full {
		return false, nil
	}
	if c.After != o.after {
		return false, execution.ErrVersionConflict
	}
	o.saved = append(o.saved, d)
	o.after = d.Sequence
	return true, nil
}
