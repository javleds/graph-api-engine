package domain

import "time"

type CachePolicy struct {
	Enabled bool
	TTL     time.Duration
}
