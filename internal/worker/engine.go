package worker

import (
	"context"
	"encoding/json"
	"errors"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	fetchapp "github.com/javleds/api-engine/internal/fetch/application"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	"github.com/javleds/api-engine/internal/worker/ports"
	"sync"
	"time"
)

// Engine executes immutable compiled definitions without API or storage dependencies.
type Engine struct {
	Fetcher     ports.Fetcher
	Progress    ports.ProgressRecorder
	Options     Options
	IdentityKey []byte
	QuotaScopes *fetchapp.QuotaScopeResolver
	FetchPolicy fetch.FetchPolicy
}

func (e *Engine) Execute(parent context.Context, definition definition.CompiledDefinition, ownerID string, id execution.ExecutionID) execution.ExecutionCompletion {
	configured := *e
	configured.Options = e.Options.defaults()
	e = &configured
	timed, cancelTimeout := context.WithTimeout(parent, e.Options.Timeout)
	defer cancelTimeout()
	ctx, cancel := context.WithCancelCause(timed)
	defer cancel(nil)
	collector := NewBoundedFailureCollector(e.Options.MaxDetails, e.Options.MaxReportBytes, e.Options.MaxMessageBytes)
	budget := &ExecutionBudget{maxLogical: e.Options.MaxLogicalRequests, maxAttempts: e.Options.MaxHTTPAttempts, maxNodes: e.Options.MaxResultNodes, maxBytes: uint64(e.Options.MaxResultBytes), cancel: cancel}
	processor := &CallProcessor{engine: e, definition: definition, ownerID: ownerID, executionID: string(id), branches: NewBranchExecutor(e.Options.MaxBranchGoroutines), budget: budget, failures: collector, progress: &progressCounters{}}
	stopProgress := make(chan struct{})
	progressFinished := make(chan struct{})
	if e.Progress != nil {
		go processor.flushProgress(ctx, stopProgress, progressFinished)
	} else {
		close(progressFinished)
	}
	outcomes := make([]NodeOutcome, len(definition.Roots))
	fatals := make([]error, len(definition.Roots))
	started := make([]bool, len(definition.Roots))
	var group sync.WaitGroup
	for index, root := range definition.Roots {
		index, root := index, root
		processor.branches.Do(&group, func() {
			started[index] = ctx.Err() == nil
			outcomes[index], fatals[index] = processor.Process(ctx, root, NodeContext{Path: root.Name, Order: []int{index}, Values: map[string]any{}})
			if fatals[index] != nil {
				cancel(fatals[index])
			}
		})
	}
	group.Wait()
	close(stopProgress)
	<-progressFinished
	var fatal *execution.ExecutionFailure
	for _, err := range fatals {
		if err != nil {
			fatal = &execution.ExecutionFailure{Code: err.Error(), Message: "No se pudo producir un resultado confiable."}
			break
		}
	}
	var global *execution.ExecutionFailure
	if fatal == nil && ctx.Err() != nil {
		code := "EXECUTION_INTERRUPTED"
		if errors.Is(context.Cause(ctx), ErrBudgetExceeded) {
			code = "EXECUTION_BUDGET_EXCEEDED"
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(context.Cause(ctx), context.DeadlineExceeded) {
			code = "EXECUTION_TIMEOUT"
		}
		global = &execution.ExecutionFailure{Code: code, Message: failureMessage(code)}
		collector.Record(execution.NodeFailure{Code: code, Message: global.Message, Scope: "execution", Stage: "execution", DescendantsSkipped: true})
	}
	roots := make([]execution.RootOutcome, 0, len(definition.Roots))
	data := make(map[string]any, len(definition.Roots))
	available := false
	for index, root := range definition.Roots {
		outcome := outcomes[index]
		status := execution.Completed
		if !started[index] {
			status = execution.ExecutionStatus("not_started")
		} else if !outcome.OwnDataAvailable {
			status = execution.Failed
		} else if outcome.Incomplete {
			status = execution.CompletedWithErrors
		}
		roots = append(roots, execution.RootOutcome{Name: root.Name, Status: status, DataAvailable: outcome.OwnDataAvailable})
		available = available || outcome.OwnDataAvailable
		if len(outcome.Value) == 0 {
			outcome = emptyOutcome(root)
		}
		data[root.Name] = outcome.Value
	}
	if fatal == nil && !available {
		fatal = global
	}
	report := collector.SnapshotWithRoots(roots)
	status, failure := execution.ResolveOutcome(report.Roots, report.ErrorSummary.Total, fatal)
	completion := execution.ExecutionCompletion{ID: id, Status: status, Error: failure, Report: report, Progress: processor.progress.snapshot(min(budget.logical.Load(), budget.maxLogical), report.ErrorSummary.Total), FinishedAt: time.Now().UTC()}
	if status != execution.Failed {
		raw, err := encodeBounded(data, e.Options.MaxResultBytes)
		if err != nil {
			completion.Status = execution.Failed
			completion.Error = &execution.ExecutionFailure{Code: err.Error(), Message: "No se pudo serializar el resultado."}
			return completion
		}
		// The API envelope adds identity, status, diagnostics and roots to the data.
		envelope, err := json.Marshal(resultEnvelope{ID: id, Status: status, Result: raw, Errors: report.Errors, ErrorSummary: report.ErrorSummary, Roots: report.Roots})
		if err != nil || len(envelope) > e.Options.MaxResultBytes {
			completion.Status = execution.Failed
			completion.Error = &execution.ExecutionFailure{Code: "RESULT_TOO_LARGE", Message: "El resultado excede el límite de bytes."}
			return completion
		}
		completion.Data = raw
	}
	return completion
}
