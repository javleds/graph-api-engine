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
	running   map[domain.ExecutionID]map[*localDispatchReservation]context.CancelFunc
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
	return &LocalExecutionDispatcher{ctx: life, cancel: cancel, runner: runner, slots: make(chan struct{}, capacity), timeout: timeout, running: make(map[domain.ExecutionID]map[*localDispatchReservation]context.CancelFunc)}
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
func (d *LocalExecutionDispatcher) CancelExecution(id domain.ExecutionID) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, cancel := range d.running[id] {
		cancel()
	}
}

func (d *LocalExecutionDispatcher) run(ctx context.Context, cancel context.CancelFunc, id domain.ExecutionID, reservation *localDispatchReservation) {
	defer func() {
		cancel()
		d.mu.Lock()
		delete(d.running[id], reservation)
		if len(d.running[id]) == 0 {
			delete(d.running, id)
		}
		d.mu.Unlock()
		<-d.slots
		d.active.Done()
	}()
	defer func() {
		if recover() != nil && d.OnFailure != nil {
			d.OnFailure(id, fmt.Errorf("INTERNAL_WORKER_PANIC"))
		}
	}()
	if err := ctx.Err(); err != nil {
		return
	}
	if err := d.runner.Run(ctx, id); err != nil && d.OnFailure != nil {
		d.OnFailure(id, err)
	}
}
