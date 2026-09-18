package clock

import "time"

type Timer interface {
	C() <-chan time.Time
	Stop() bool
}
