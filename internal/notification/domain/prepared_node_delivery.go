package domain

import (
	"time"
)

type PreparedNodeDelivery struct {
	WebhookDelivery
	Sequence   int64
	OccurredAt time.Time
	Failure    string
}
