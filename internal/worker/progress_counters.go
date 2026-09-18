package worker

import (
	execution "github.com/javleds/api-engine/internal/execution/domain"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	"sync/atomic"
)

type progressCounters struct{ attempts, cacheHits, coalesced, retries, rootItems, skipped atomic.Uint64 }

func (p *progressCounters) observe(result fetch.FetchResult) {
	if result.CacheHit {
		p.cacheHits.Add(1)
	}
	if result.ReusedInFlight {
		p.coalesced.Add(1)
	}
}
func (p *progressCounters) snapshot(logical, failures uint64) execution.ExecutionProgress {
	return execution.ExecutionProgress{LogicalRequests: logical, HTTPAttempts: p.attempts.Load(), CacheHits: p.cacheHits.Load(), CoalescedRequests: p.coalesced.Load(), Retries: p.retries.Load(), NodeFailures: failures, CompletedRootItems: p.rootItems.Load(), SkippedSubtrees: p.skipped.Load()}
}
