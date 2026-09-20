package domain

import "time"

// V2Result is the durable NodeFinished event for one complete root result.
// StorageVersion selects either the legacy chunk range or immutable node
// projection, without copying the subtree into the event.
type V2Result struct {
	Version        int          `json:"version"`
	EventID        string       `json:"eventId"`
	ExecutionID    ExecutionID  `json:"executionId"`
	NodeID         string       `json:"nodeId"`
	Sequence       string       `json:"sequence"`
	Type           string       `json:"type"`
	Root           string       `json:"root"`
	Path           string       `json:"path"`
	Index          *int64       `json:"index,omitempty"`
	Status         string       `json:"status"`
	OccurredAt     time.Time    `json:"occurredAt"`
	ErrorSummary   ErrorSummary `json:"errorSummary"`
	FirstChunk     int64        `json:"-"`
	LastChunk      int64        `json:"-"`
	StorageVersion int          `json:"-"`
	SourceRoot     int          `json:"-"`
}
