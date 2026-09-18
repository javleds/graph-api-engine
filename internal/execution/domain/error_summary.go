package domain

type ErrorSummary struct {
	Total     uint64 `json:"total"`
	Returned  uint64 `json:"returned"`
	Truncated bool   `json:"truncated"`
}
