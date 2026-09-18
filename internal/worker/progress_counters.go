package worker

import (
	execution "github.com/javleds/api-engine/internal/execution/domain"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	"sync/atomic"
)

type progressCounters struct{ attempts, cacheHits, coalesced, retries, rootItems, skipped, successfulRoots, partialRoots, failedRoots atomic.Uint64 }

func (p *progressCounters) observe(result fetch.FetchResult) {
	if result.CacheHit {
		p.cacheHits.Add(1)
	}
	if result.ReusedInFlight {
		p.coalesced.Add(1)
	}
}
func (p *progressCounters) snapshot(logical, failures uint64) execution.ExecutionProgress {
	return execution.ExecutionProgress{LogicalRequests: logical, HTTPAttempts: p.attempts.Load(), CacheHits: p.cacheHits.Load(), CoalescedRequests: p.coalesced.Load(), Retries: p.retries.Load(), NodeFailures: failures, CompletedRootItems: p.rootItems.Load(), SkippedSubtrees: p.skipped.Load(), SuccessfulRootCalls: p.successfulRoots.Load(), PartialRootCalls: p.partialRoots.Load(), FailedRootCalls: p.failedRoots.Load()}
}

func (p *progressCounters) rootCompleted(outcome NodeOutcome) {
	if !outcome.OwnDataAvailable {
		p.failedRoots.Add(1)
	} else if outcome.Incomplete {
		p.partialRoots.Add(1)
	} else {
		p.successfulRoots.Add(1)
	}
}
