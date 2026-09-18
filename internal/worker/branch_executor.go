package worker

import "sync"

// BranchExecutor never waits for a slot while its caller owns another slot.
type BranchExecutor struct{ slots chan struct{} }

func NewBranchExecutor(limit int) *BranchExecutor {
	if limit < 0 {
		limit = 0
	}
	return &BranchExecutor{slots: make(chan struct{}, limit)}
}
func (b *BranchExecutor) Do(group *sync.WaitGroup, run func()) {
	select {
	case b.slots <- struct{}{}:
		group.Add(1)
		go func() { defer group.Done(); defer func() { <-b.slots }(); run() }()
	default:
		run()
	}
}
