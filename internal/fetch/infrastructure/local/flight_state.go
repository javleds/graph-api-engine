package local

import "context"

type flightState struct {
	context     context.Context
	cancel      context.CancelFunc
	done        chan struct{}
	subscribers int
	generation  string
}
