package domain

import "time"

type Execution struct {
	APIVersion      int
	ID              ExecutionID
	OwnerID         string
	Fingerprint     string
	Status          ExecutionStatus
	Version         int64
	CreatedAt       time.Time
	StartedAt       *time.Time
	FinishedAt      *time.Time
	ResultExpiresAt *time.Time
	Progress        ExecutionProgress
	Error           *ExecutionFailure
	Report          ExecutionErrorReport
	WebhookEnabled  bool
}
