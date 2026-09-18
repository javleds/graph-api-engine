package worker

import (
	"context"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	"time"
)

func (p *CallProcessor) flushProgress(ctx context.Context, stop <-chan struct{}, finished chan<- struct{}) {
	defer close(finished)
	ticker := time.NewTicker(p.engine.Options.ProgressFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			report := p.failures.Snapshot()
			progress := p.progress.snapshot(min(p.budget.logical.Load(), p.budget.maxLogical), report.ErrorSummary.Total)
			flush, cancel := context.WithTimeout(ctx, p.engine.Options.PersistenceTimeout)
			_ = p.engine.Progress.RecordProgress(flush, execution.ExecutionID(p.executionID), progress)
			cancel()
		}
	}
}
