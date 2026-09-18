package domain

import "time"

type WebhookRecord struct {
	ID         string
	EventID    string
	Target     []byte
	Payload    []byte
	DeadlineAt time.Time
}
