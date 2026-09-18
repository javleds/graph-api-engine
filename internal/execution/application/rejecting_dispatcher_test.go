package application

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/application/ports"
	"github.com/javleds/api-engine/internal/execution/domain"
)

type rejectingDispatcher struct{}

func (rejectingDispatcher) Reserve(context.Context) (ports.DispatchReservation, error) {
	return nil, domain.ErrEngineBusy
}
