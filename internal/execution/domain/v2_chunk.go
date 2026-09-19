package domain

import (
	"encoding/json"
	"time"
)

type V2Chunk struct {
	Version     int             `json:"version"`
	ExecutionID ExecutionID     `json:"executionId"`
	Sequence    string          `json:"sequence"`
	CreatedAt   time.Time       `json:"createdAt"`
	Type        string          `json:"type"`
	Operation   string          `json:"operation,omitempty"`
	NodeID      string          `json:"nodeId,omitempty"`
	ParentID    string          `json:"parentId,omitempty"`
	Key         string          `json:"key,omitempty"`
	Index       *int64          `json:"index,omitempty"`
	Path        string          `json:"path,omitempty"`
	Value       json.RawMessage `json:"value,omitempty"`
	Status      string          `json:"status,omitempty"`
}
