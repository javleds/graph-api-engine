package domain

import (
	executiondomain "github.com/javleds/api-engine/internal/execution/domain"
	"time"
)

type WebhookDelivery struct {
	ID             string
	EventID        string
	ExecutionID    executiondomain.ExecutionID
	OwnerID        string
	EventType      string
	Target         []byte
	Payload        []byte
	Status         string
	Attempts       int
	Version        int64
	LeaseOwner     string
	LeaseExpiresAt time.Time
	NextAttemptAt  time.Time
	DeadlineAt     time.Time
}
