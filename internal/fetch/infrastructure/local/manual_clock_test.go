package local

import (
	"github.com/javleds/api-engine/internal/platform/clock"
	"sync"
	"time"
)

type manualClock struct {
	mu     sync.Mutex
	now    time.Time
	timers map[*manualTimer]bool
}

func (c *manualClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *manualClock) NewTimer(d time.Duration) clock.Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	timer := &manualTimer{clock: c, deadline: c.now.Add(d), channel: make(chan time.Time, 1)}
	if c.timers == nil {
		c.timers = make(map[*manualTimer]bool)
	}
	if d <= 0 {
		timer.channel <- c.now
	} else {
		c.timers[timer] = true
	}
	return timer
}
func (c *manualClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	for timer := range c.timers {
		if !timer.deadline.After(c.now) {
			timer.channel <- c.now
			delete(c.timers, timer)
		}
	}
}
