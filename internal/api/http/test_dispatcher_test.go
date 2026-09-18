package http_test

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/application/ports"
	"github.com/javleds/api-engine/internal/execution/domain"
)

type testDispatcher struct {
	busy    bool
	commits int
}

func (d *testDispatcher) Reserve(context.Context) (ports.DispatchReservation, error) {
	if d.busy {
		return nil, domain.StorageError("ENGINE_BUSY")
	}
	return &testReservation{dispatcher: d}, nil
}
