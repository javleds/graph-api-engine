package bootstrap

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/application/ports"
	"github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/platform/observability"
	"time"
)

type measuredRunner struct {
	runner  ports.ExecutionRunner
	store   ports.ExecutionStore
	metrics *observability.Metrics
}

func (r measuredRunner) Run(ctx context.Context, id domain.ExecutionID) error {
	start := time.Now()
	r.metrics.Add("executions_active", 1)
	defer r.metrics.Add("executions_active", -1)
	defer func() { r.metrics.Observe("execution_duration_seconds", time.Since(start).Seconds()) }()
	err := r.runner.Run(ctx, id)
	query, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	_, execution, readErr := r.store.Load(query, id)
	if readErr == nil && execution.Status.Terminal() {

		p := execution.Progress
		r.metrics.Add("root_calls_completed_total", float64(p.SuccessfulRootCalls))
		r.metrics.Add("root_calls_partial_total", float64(p.PartialRootCalls))
		r.metrics.Add("root_calls_failed_total", float64(p.FailedRootCalls))
		r.metrics.Add("logical_requests_total", float64(p.LogicalRequests))
		r.metrics.Add("cache_hits_total", float64(p.CacheHits))
		r.metrics.Add("coalesced_requests_total", float64(p.CoalescedRequests))

		r.metrics.Add("node_failures_total", float64(p.NodeFailures))
		r.metrics.Add("skipped_subtrees_total", float64(p.SkippedSubtrees))
	}
	return err
}
