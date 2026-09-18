package domain

import "time"

type FetchPolicy struct {
	CacheEnabled     bool
	CacheTTL         time.Duration
	AttemptTimeout   time.Duration
	OperationTimeout time.Duration
	MaxAttempts      int
	BaseDelay        time.Duration
	MaxBackoff       time.Duration
}

func (p FetchPolicy) WithDefaults() FetchPolicy {
	if p.AttemptTimeout <= 0 {
		p.AttemptTimeout = 30 * time.Second
	}
	if p.OperationTimeout <= 0 {
		p.OperationTimeout = 30 * time.Minute
	}
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 4
	}
	if p.BaseDelay <= 0 {
		p.BaseDelay = 500 * time.Millisecond
	}
	if p.MaxBackoff <= 0 {
		p.MaxBackoff = 10 * time.Second
	}
	return p
}
