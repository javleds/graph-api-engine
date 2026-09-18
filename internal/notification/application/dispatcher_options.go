package application

import "time"

type DispatcherOptions struct {
	InstanceID         string
	MaxConcurrent      int
	PollInterval       time.Duration
	ClaimLease         time.Duration
	AttemptTimeout     time.Duration
	PersistenceTimeout time.Duration
	MaxAttempts        int
	BaseDelay          time.Duration
	MaxBackoff         time.Duration
	Jitter             func(time.Duration) time.Duration
}

func (o DispatcherOptions) defaults() DispatcherOptions {
	if o.MaxConcurrent <= 0 {
		o.MaxConcurrent = 4
	}
	if o.PollInterval <= 0 {
		o.PollInterval = time.Second
	}
	if o.ClaimLease <= 0 {
		o.ClaimLease = 30 * time.Second
	}
	if o.AttemptTimeout <= 0 {
		o.AttemptTimeout = 10 * time.Second
	}
	if o.PersistenceTimeout <= 0 {
		o.PersistenceTimeout = 5 * time.Second
	}
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = 8
	}
	if o.BaseDelay <= 0 {
		o.BaseDelay = 5 * time.Second
	}
	if o.MaxBackoff <= 0 {
		o.MaxBackoff = 5 * time.Minute
	}
	return o
}
