package domain

import "time"

type ExecutionCompletion struct {
	ID              ExecutionID
	ExpectedVersion int64
	Status          ExecutionStatus
	Error           *ExecutionFailure
	Report          ExecutionErrorReport
	Data            []byte
	Progress        ExecutionProgress
	FinishedAt      time.Time
	ResultExpiresAt time.Time
	Delivery        *WebhookRecord
}
