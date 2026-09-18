package local

import (
	"context"
	"errors"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func awaitSubscribers(t *testing.T, c *LocalFlightCoordinator, key string, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		state := c.flights[key]
		count := 0
		if state != nil {
			count = state.subscribers
		}
		c.mu.Unlock()
		if count == want {
			return
		}
		runtime.Gosched()
	}
	t.Fatal("subscriber count did not converge")
}

func TestFlightCancelLeaderKeepsFollowerAndCopiesResponse(t *testing.T) {
	c := NewLocalFlightCoordinator(10, time.Minute)
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	work := func(ctx context.Context) (domain.RawResponse, error) {
		calls.Add(1)
		close(started)
		select {
		case <-ctx.Done():
			return domain.RawResponse{}, ctx.Err()
		case <-release:
			return domain.RawResponse{Body: []byte(`{}`)}, nil
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	leader := make(chan error, 1)
	go func() { _, err := c.Do(ctx, "key", work); leader <- err }()
	<-started
	follower := make(chan domain.FlightResult, 1)
	go func() {
		result, err := c.Do(context.Background(), "key", work)
		if err != nil {
			t.Error(err)
		}
		follower <- result
	}()
	awaitSubscribers(t, c, "key", 2)
	cancel()
	if err := <-leader; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(release)
	result := <-follower
	if !result.ReusedInFlight || calls.Load() != 1 || string(result.Response.Body) != "{}" {
		t.Fatalf("%+v calls %d", result, calls.Load())
	}
}

func TestFlightAllCancelRetainsGenerationUntilCallbackEnds(t *testing.T) {
	c := NewLocalFlightCoordinator(1, time.Minute)
	started := make(chan struct{})
	canceled := make(chan struct{})
	finish := make(chan struct{})
	firstDone := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer close(firstDone)
		_, _ = c.Do(ctx, "key", func(ctx context.Context) (domain.RawResponse, error) {
			close(started)
			<-ctx.Done()
			close(canceled)
			<-finish
			return domain.RawResponse{}, ctx.Err()
		})
	}()
	<-started
	cancel()
	<-firstDone
	<-canceled
	newStarted := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		_, err := c.Do(context.Background(), "key", func(context.Context) (domain.RawResponse, error) {
			close(newStarted)
			return domain.RawResponse{Body: []byte(`[]`)}, nil
		})
		secondDone <- err
	}()
	select {
	case <-newStarted:
		t.Fatal("overlapping generation")
	default:
	}
	c.mu.Lock()
	if len(c.flights) != 1 {
		t.Error("canceling callback removed early")
	}
	c.mu.Unlock()
	close(finish)
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
}

func TestFlightCapacityAndPanicCleanup(t *testing.T) {
	c := NewLocalFlightCoordinator(1, time.Minute)
	release := make(chan struct{})
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := c.Do(context.Background(), "a", func(context.Context) (domain.RawResponse, error) { close(started); <-release; panic("secret") })
		done <- err
	}()
	<-started
	_, err := c.Do(context.Background(), "b", func(context.Context) (domain.RawResponse, error) {
		t.Fatal("capacity bypass")
		return domain.RawResponse{}, nil
	})
	if err == nil || err.Error() != "FETCH_CAPACITY_EXCEEDED" {
		t.Fatalf("%v", err)
	}
	close(release)
	if err = <-done; err == nil || err.Error() != "INTERNAL_WORKER_PANIC" {
		t.Fatalf("%v", err)
	}
	_, err = c.Do(context.Background(), "a", func(context.Context) (domain.RawResponse, error) { return domain.RawResponse{}, nil })
	if err != nil {
		t.Fatal(err)
	}
}
