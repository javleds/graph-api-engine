package domain

import "time"

type QuotaRule struct {
	Scope       string
	Requests    int
	Period      time.Duration
	Concurrency int
}
