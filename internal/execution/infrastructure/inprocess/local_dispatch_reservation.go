package inprocess

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/domain"
	"sync"
)

type localDispatchReservation struct {
	mu         sync.Mutex
	used       bool
	dispatcher *LocalExecutionDispatcher
}

func (r *localDispatchReservation) Commit(_ context.Context, id domain.ExecutionID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.used {
		return domain.StorageError("DISPATCH_FAILED")
	}
	r.dispatcher.mu.Lock()
	defer r.dispatcher.mu.Unlock()
	if r.dispatcher.closed {
		return domain.StorageError("DISPATCH_FAILED")
	}
	r.used = true
	ctx, cancel := context.WithCancel(r.dispatcher.ctx)
	if r.dispatcher.timeout > 0 {
		cancel()
		ctx, cancel = context.WithTimeout(r.dispatcher.ctx, r.dispatcher.timeout)
	}
	if r.dispatcher.running[id] == nil {
		r.dispatcher.running[id] = make(map[*localDispatchReservation]context.CancelFunc)
	}
	r.dispatcher.running[id][r] = cancel
	go r.dispatcher.run(ctx, cancel, id, r)
	return nil
}
func (r *localDispatchReservation) Cancel() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.used {
		return
	}
	r.used = true
	<-r.dispatcher.slots
	r.dispatcher.active.Done()
}
