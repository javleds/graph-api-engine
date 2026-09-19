package config

import execution "github.com/javleds/api-engine/internal/execution/domain"

// V2Config bounds durable data independently of v1's in-memory result budget.
type V2Config struct {
	MaxPublishedBytes int64 `json:"maxPublishedBytes"`
	MaxStoredBytes    int64 `json:"maxStoredBytes"`
	MaxChunks         int64 `json:"maxChunks"`
	MaxNodes          int64 `json:"maxNodes"`
	MaxPendingTasks   int64 `json:"maxPendingTasks"`
}

func (c V2Config) Limits() execution.V2Limits {
	return execution.V2Limits{MaxPublishedBytes: c.MaxPublishedBytes, MaxStoredBytes: c.MaxStoredBytes, MaxChunks: c.MaxChunks, MaxNodes: c.MaxNodes, MaxPendingTasks: c.MaxPendingTasks}
}
