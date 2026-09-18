package http

import "time"

type SenderOptions struct {
	AttemptTimeout   time.Duration
	ConnectTimeout   time.Duration
	MaxResponseBytes int64
	MaxConcurrent    int
}
