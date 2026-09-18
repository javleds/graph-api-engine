package sqlite

import (
	"github.com/javleds/api-engine/internal/platform/clock"
	"time"
)

type fixedClock struct{ now time.Time }

func (c *fixedClock) Now() time.Time                       { return c.now }
func (c *fixedClock) NewTimer(d time.Duration) clock.Timer { return clock.RealClock{}.NewTimer(d) }
