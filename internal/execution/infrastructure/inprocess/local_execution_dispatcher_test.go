package inprocess

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/domain"
	"testing"
	"time"
)

func TestReservationCapacityAndRequestIndependence(t *testing.T) {
	runner := &blockingRunner{started: make(chan struct{}), release: make(chan struct{})}
	dispatcher := NewLocalExecutionDispatcher(context.Background(), runner, 1, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	reservation, err := dispatcher.Reserve(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dispatcher.Reserve(ctx); err == nil {
		t.Fatal("unbounded capacity")
	}
	if err := reservation.Commit(ctx, domain.ExecutionID("test")); err != nil {
		t.Fatal(err)
	}
	cancel()
	<-runner.started
	reservation.Cancel()
	if _, err := dispatcher.Reserve(context.Background()); err == nil {
		t.Fatal("cancel released committed slot")
	}
	close(runner.release)
	wait, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err := dispatcher.Wait(wait); err != nil {
		t.Fatal(err)
	}
	dispatcher.StopAdmission()
	if _, err := dispatcher.Reserve(context.Background()); err == nil {
		t.Fatal("shutdown admits")
	}
}
