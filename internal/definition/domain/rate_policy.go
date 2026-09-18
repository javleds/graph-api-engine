package domain

import "time"

type RatePolicy struct {
	Requests    int64
	Per         time.Duration
	Concurrency int
}
