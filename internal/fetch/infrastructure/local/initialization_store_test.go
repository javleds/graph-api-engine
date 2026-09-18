package local

import (
	"context"
	"errors"
	"sync"
	"time"
)

type initializationStore struct {
	mu        sync.Mutex
	calls     int
	failFirst bool
	entered   chan struct{}
	release   chan struct{}
	until     time.Time
}

func (s *initializationStore) Get(ctx context.Context, _ string) (time.Time, error) {
	s.mu.Lock()
	s.calls++
	first := s.calls == 1
	s.mu.Unlock()
	if first && s.entered != nil {
		close(s.entered)
		select {
		case <-ctx.Done():
			return time.Time{}, ctx.Err()
		case <-s.release:
		}
	}
	if first && s.failFirst {
		return time.Time{}, errors.New("temporary read failure")
	}
	return s.until, nil
}
func (s *initializationStore) Extend(context.Context, string, time.Time) error { return nil }
func (s *initializationStore) DeleteExpired(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}
func (s *initializationStore) readCount() int { s.mu.Lock(); defer s.mu.Unlock(); return s.calls }
