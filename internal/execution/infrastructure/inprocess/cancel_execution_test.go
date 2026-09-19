package inprocess

import (
	"context"
	"testing"
	"time"

	"github.com/javleds/api-engine/internal/execution/domain"
)

func TestCancelExecutionIsolatesJobsAndCleansRegistrations(t *testing.T) {
	runner := &cancelRunner{started: make(chan domain.ExecutionID, 3), stopped: make(chan domain.ExecutionID, 3)}
	dispatcher := NewLocalExecutionDispatcher(context.Background(), runner, 3, time.Minute)
	defer dispatcher.Cancel()
	for _, id := range []domain.ExecutionID{"first", "first", "other"} {
		reservation, err := dispatcher.Reserve(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if err = reservation.Commit(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ {
		select {
		case <-runner.started:
		case <-time.After(time.Second):
			t.Fatal("not started")
		}
	}
	dispatcher.CancelExecution("first")
	for i := 0; i < 2; i++ {
		select {
		case id := <-runner.stopped:
			if id != "first" {
				t.Fatal("cancelled other job")
			}
		case <-time.After(time.Second):
			t.Fatal("not cancelled")
		}
	}
	dispatcher.mu.Lock()
	if len(dispatcher.running["other"]) != 1 {
		t.Fatal("lost other registration")
	}
	dispatcher.mu.Unlock()
	dispatcher.CancelExecution("other")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := dispatcher.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	dispatcher.mu.Lock()
	defer dispatcher.mu.Unlock()
	if len(dispatcher.running) != 0 || len(dispatcher.slots) != 0 {
		t.Fatal("leaked work registrations")
	}
}

func TestCommitRegistersCancellationSynchronously(t *testing.T) {
	runner := &cancelRunner{started: make(chan domain.ExecutionID, 1), stopped: make(chan domain.ExecutionID, 1)}
	dispatcher := NewLocalExecutionDispatcher(context.Background(), runner, 1, time.Minute)
	defer dispatcher.Cancel()
	reservation, err := dispatcher.Reserve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err = reservation.Commit(context.Background(), "pending"); err != nil {
		t.Fatal(err)
	}
	dispatcher.mu.Lock()
	if len(dispatcher.running["pending"]) != 1 {
		t.Fatal("missing synchronous registration")
	}
	dispatcher.mu.Unlock()
	dispatcher.CancelExecution("pending")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := dispatcher.Wait(ctx); err != nil {
		t.Fatal(err)
	}
}
