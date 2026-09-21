package config

import execution "github.com/javleds/api-engine/internal/execution/domain"

// V2Config bounds durable data independently of v1's in-memory result budget.
type V2Config struct {
	MaxPublishedBytes int64 `json:"maxPublishedBytes" yaml:"maxPublishedBytes" validate:"gt=0"`
	MaxStoredBytes    int64 `json:"maxStoredBytes" yaml:"maxStoredBytes" validate:"gt=0"`
	MaxChunks         int64 `json:"maxChunks" yaml:"maxChunks" validate:"gt=0"`
	MaxNodes          int64 `json:"maxNodes" yaml:"maxNodes" validate:"gt=0"`
	MaxPendingTasks   int64 `json:"maxPendingTasks" yaml:"maxPendingTasks" validate:"gt=0"`
}

func (c V2Config) Limits() execution.V2Limits {
	return execution.V2Limits{MaxPublishedBytes: c.MaxPublishedBytes, MaxStoredBytes: c.MaxStoredBytes, MaxChunks: c.MaxChunks, MaxNodes: c.MaxNodes, MaxPendingTasks: c.MaxPendingTasks}
}
