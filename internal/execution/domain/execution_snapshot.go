package domain

import "time"

type ExecutionSnapshot struct {
	ID                 ExecutionID
	OwnerID            string
	Fingerprint        string
	IdempotencyKeyHash string
	Definition         []byte
	WebhookEnabled     bool
	CreatedAt          time.Time
	MetadataExpiresAt  time.Time
}
