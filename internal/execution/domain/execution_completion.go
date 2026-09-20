package domain

import "time"

type ExecutionCompletion struct {
	ExpectedCheckpointRevision *int64
	APIVersion                 int
	LastSequence               string
	LastResultSequence         string
	ResultCount                int64
	ExecutionBasePath          string
	ID                         ExecutionID
	ExpectedVersion            int64
	Status                     ExecutionStatus
	Error                      *ExecutionFailure
	Report                     ExecutionErrorReport
	Data                       []byte
	Progress                   ExecutionProgress
	FinishedAt                 time.Time
	ResultExpiresAt            time.Time
	Delivery                   *WebhookRecord
}
