package domain

import "time"

type WebhookAttempt struct {
	StatusCode int
	Retryable  bool
	RetryAfter time.Time
	ErrorCode  string
}

func (a WebhookAttempt) Accepted() bool {
	return a.StatusCode >= 200 && a.StatusCode < 300 && a.ErrorCode == ""
}
