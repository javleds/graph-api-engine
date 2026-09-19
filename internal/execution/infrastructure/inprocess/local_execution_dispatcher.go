package inprocess

import (
	"context"
	"fmt"
	"github.com/javleds/api-engine/internal/execution/application/ports"
	"github.com/javleds/api-engine/internal/execution/domain"
	"sync"
	"time"
)

type LocalExecutionDispatcher struct {
	mu        sync.Mutex
	closed    bool
	slots     chan struct{}
	active    sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	runner    ports.ExecutionRunner
	timeout   time.Duration
	OnFailure func(domain.ExecutionID, error)
}

func NewLocalExecutionDispatcher(ctx context.Context, runner ports.ExecutionRunner, capacity int, timeout time.Duration) *LocalExecutionDispatcher {
	life, cancel := context.WithCancel(ctx)
	return &LocalExecutionDispatcher{ctx: life, cancel: cancel, runner: runner, slots: make(chan struct{}, capacity), timeout: timeout}
}
func (d *LocalExecutionDispatcher) Reserve(ctx context.Context) (ports.DispatchReservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil, domain.StorageError("ENGINE_BUSY")
	}
	select {
	case d.slots <- struct{}{}:
		d.active.Add(1)
		return &localDispatchReservation{dispatcher: d}, nil
	default:
		return nil, domain.StorageError("ENGINE_BUSY")
	}
}
func (d *LocalExecutionDispatcher) StopAdmission() { d.mu.Lock(); d.closed = true; d.mu.Unlock() }
func (d *LocalExecutionDispatcher) Cancel()        { d.cancel() }
func (d *LocalExecutionDispatcher) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() { d.active.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (d *LocalExecutionDispatcher) run(id domain.ExecutionID) {
	defer func() { <-d.slots; d.active.Done() }()
	defer func() {
		if recover() != nil && d.OnFailure != nil {
			d.OnFailure(id, fmt.Errorf("INTERNAL_WORKER_PANIC"))
		}
	}()
	ctx, cancel := context.WithCancel(d.ctx)
	if d.timeout > 0 {
		cancel()
		ctx, cancel = context.WithTimeout(d.ctx, d.timeout)
	}
	defer cancel()
	if err := d.runner.Run(ctx, id); err != nil && d.OnFailure != nil {
		d.OnFailure(id, err)
	}
}
