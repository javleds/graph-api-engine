package local

import "context"

type flightState struct {
	subscriptions []*flightSubscription
	context       context.Context
	cancel        context.CancelFunc
	done          chan struct{}
	subscribers   int
	generation    string
}
