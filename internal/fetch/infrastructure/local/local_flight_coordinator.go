package local

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"golang.org/x/sync/singleflight"
	"strconv"
	"sync"
	"time"
)

type LocalFlightCoordinator struct {
	mu               sync.Mutex
	group            singleflight.Group
	flights          map[string]*flightState
	sequence         uint64
	maxActive        int
	maxOperationTime time.Duration
}

func NewLocalFlightCoordinator(maxActive int, maxOperationTime time.Duration) *LocalFlightCoordinator {
	if maxActive <= 0 {
		maxActive = 128
	}
	if maxOperationTime <= 0 {
		maxOperationTime = 30 * time.Minute
	}
	return &LocalFlightCoordinator{flights: make(map[string]*flightState), maxActive: maxActive, maxOperationTime: maxOperationTime}
}

func (c *LocalFlightCoordinator) Do(ctx context.Context, key string, work func(context.Context) (domain.RawResponse, error)) (domain.FlightResult, error) {
	for {
		if err := ctx.Err(); err != nil {
			return domain.FlightResult{}, err
		}
		c.mu.Lock()
		state, exists := c.flights[key]
		if exists && state.context.Err() != nil {
			done := state.done
			c.mu.Unlock()
			select {
			case <-ctx.Done():
				return domain.FlightResult{}, ctx.Err()
			case <-done:
				continue
			}
		}
		if !exists {
			if len(c.flights) >= c.maxActive {
				c.mu.Unlock()
				return domain.FlightResult{}, &domain.UpstreamFailure{Code: "FETCH_CAPACITY_EXCEEDED"}
			}
			workCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.maxOperationTime)
			c.sequence++
			state = &flightState{context: workCtx, cancel: cancel, done: make(chan struct{}), generation: strconv.FormatUint(c.sequence, 10)}
			c.flights[key] = state
		}
		state.subscribers++
		result := c.group.DoChan(state.generation, func() (any, error) { return c.run(key, state, work) })
		c.mu.Unlock()
		return c.await(ctx, key, state, result, exists)
	}
}

func (c *LocalFlightCoordinator) run(key string, state *flightState, work func(context.Context) (domain.RawResponse, error)) (response domain.RawResponse, err error) {
	defer func() {
		if recover() != nil {
			response = domain.RawResponse{}
			err = &domain.UpstreamFailure{Code: "INTERNAL_WORKER_PANIC"}
		}
		state.cancel()
		c.mu.Lock()
		if c.flights[key] == state {
			delete(c.flights, key)
		}
		close(state.done)
		c.mu.Unlock()
	}()
	return work(state.context)
}

func (c *LocalFlightCoordinator) await(ctx context.Context, key string, state *flightState, result <-chan singleflight.Result, reused bool) (domain.FlightResult, error) {
	defer func() {
		c.mu.Lock()
		state.subscribers--
		if state.subscribers == 0 {
			state.cancel()
		}
		c.mu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return domain.FlightResult{}, ctx.Err()
	case completed := <-result:
		if completed.Err != nil {
			if failure, ok := completed.Err.(*domain.UpstreamFailure); ok {
				copy := *failure
				return domain.FlightResult{ReusedInFlight: reused}, &copy
			}
			return domain.FlightResult{ReusedInFlight: reused}, completed.Err
		}
		return domain.FlightResult{Response: completed.Val.(domain.RawResponse).Clone(), ReusedInFlight: reused}, nil
	}
}

func (c *LocalFlightCoordinator) ActiveFlights() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.flights)
}
