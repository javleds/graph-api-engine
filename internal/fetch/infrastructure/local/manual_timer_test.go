package local

import "time"

type manualTimer struct {
	clock    *manualClock
	deadline time.Time
	channel  chan time.Time
}

func (t *manualTimer) C() <-chan time.Time { return t.channel }
func (t *manualTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	active := t.clock.timers[t]
	delete(t.clock.timers, t)
	return active
}
