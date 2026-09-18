package http_test

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/domain"
)

type testReservation struct{ dispatcher *testDispatcher }

func (r *testReservation) Commit(context.Context, domain.ExecutionID) error {
	r.dispatcher.commits++
	return nil
}
func (r *testReservation) Cancel() {}
