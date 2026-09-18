package local

import "time"

type GateOptions struct {
	MaxConcurrent    int
	MaxScopes        int
	Utilization      float64
	ProcessStartedAt time.Time
	AttemptTimeout   time.Duration
	SafetyMargin     time.Duration
}
