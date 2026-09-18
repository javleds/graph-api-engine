package domain

type ExecutionProgress struct {
	LogicalRequests    uint64 `json:"logicalRequests"`
	HTTPAttempts       uint64 `json:"httpAttempts"`
	CacheHits          uint64 `json:"cacheHits"`
	CoalescedRequests  uint64 `json:"coalescedRequests"`
	Retries            uint64 `json:"retries"`
	CompletedRootItems uint64 `json:"completedRootItems"`
	NodeFailures       uint64 `json:"nodeFailures"`
	SkippedSubtrees    uint64 `json:"skippedSubtrees"`
}
