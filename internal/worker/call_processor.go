package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	fetchdomain "github.com/javleds/api-engine/internal/fetch/domain"
	"io"
)

// CallProcessor is execution-scoped; branch outputs have exactly one writer.
type CallProcessor struct {
	engine               *Engine
	definition           definition.CompiledDefinition
	ownerID, executionID string
	branches             *BranchExecutor
	budget               *ExecutionBudget
	failures             *BoundedFailureCollector
	progress             *progressCounters
}

func emptyOutcome(call definition.CompiledCall) NodeOutcome {
	if call.Type == "collection" {
		return NodeOutcome{Value: json.RawMessage("[]")}
	}
	return NodeOutcome{Value: json.RawMessage("null")}
}
func (p *CallProcessor) failure(call definition.CompiledCall, n NodeContext, code, stage string, attempts int) {
	p.recordFailure(nodeFailure(call, n, code, stage, attempts), n.Order)
}
func (p *CallProcessor) Process(ctx context.Context, call definition.CompiledCall, n NodeContext) (outcome NodeOutcome, fatal error) {
	outcome = emptyOutcome(call)
	defer func() {
		if recover() != nil {
			fatal = errors.New("INTERNAL_WORKER_PANIC")
			p.budget.cancel(fatal)
		}
	}()
	if ctx.Err() != nil {
		return outcome, nil
	}
	if call.Pagination != nil {
		return p.paginate(ctx, call, n)
	}
	value, attempts, ok, err := p.fetch(ctx, call, n)
	if err != nil {
		return outcome, err
	}
	if !ok {
		outcome.Incomplete = true
		if ctx.Err() == nil {
			outcome.FailureCount = 1
		}
		return outcome, nil
	}
	outcome.OwnDataAvailable = true
	value, count, err := p.enrich(ctx, call, n, value)
	if err != nil {
		return outcome, err
	}
	outcome.FailureCount = count
	outcome.Incomplete = count > 0 || ctx.Err() != nil
	outcome.Value, err = encodeBounded(value, p.engine.Options.MaxResultBytes)
	if err != nil {
		return outcome, err
	}
	_ = attempts
	return outcome, nil
}
func (p *CallProcessor) fetch(ctx context.Context, call definition.CompiledCall, n NodeContext) (any, int, bool, error) {
	if ctx.Err() != nil || !p.budget.request() {
		return nil, 0, false, nil
	}
	request, err := p.buildRequest(ctx, call, n)
	if err != nil {
		var invalid *definition.ValidationError
		if !errors.As(err, &invalid) {
			fatal := errors.New("INTERNAL_WORKER_ERROR")
			p.budget.cancel(fatal)
			return nil, 0, false, fatal
		}
		p.recordFailure(mapFetchFailure(call, n, err), n.Order)
		return nil, 0, false, nil
	}
	response, err := p.engine.Fetcher.Fetch(ctx, request)
	if err != nil {
		var upstream *fetchdomain.UpstreamFailure
		if errors.As(err, &upstream) && upstream.Code == "INTERNAL_WORKER_PANIC" {
			p.budget.cancel(err)
			return nil, 0, false, errors.New("INTERNAL_WORKER_PANIC")
		}
		if ctx.Err() == nil && !errors.As(err, &upstream) && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			code := "INTERNAL_WORKER_ERROR"
			if err.Error() == "STORAGE_UNAVAILABLE" {
				code = "STORAGE_UNAVAILABLE"
			}
			if err.Error() == "FETCH_CAPACITY_EXCEEDED" {
				p.failure(call, n, "FETCH_CAPACITY_EXCEEDED", "fetch", 0)
				return nil, 0, false, nil
			}
			fatal := errors.New(code)
			p.budget.cancel(fatal)
			return nil, 0, false, fatal
		}
		if ctx.Err() == nil {
			p.recordFailure(mapFetchFailure(call, n, err), n.Order)
		}
		return nil, 0, false, nil
	}
	p.progress.observe(response)
	decoder := json.NewDecoder(bytes.NewReader(response.Response.Body))
	decoder.UseNumber()
	var value any
	if err = decoder.Decode(&value); err != nil {
		p.failure(call, n, "UPSTREAM_INVALID_JSON", "fetch", response.Response.Attempts)
		return nil, 0, false, nil
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		p.failure(call, n, "UPSTREAM_INVALID_JSON", "fetch", response.Response.Attempts)
		return nil, 0, false, nil
	}
	selected, err := definition.Select(value, call.Select)
	if err != nil {
		p.failure(call, n, "SELECT_PATH_NOT_FOUND", "select", response.Response.Attempts)
		return nil, 0, false, nil
	}
	_, array := selected.([]any)
	if (call.Type == "collection") != array {
		p.failure(call, n, "TYPE_MISMATCH", "type", response.Response.Attempts)
		return nil, 0, false, nil
	}
	return selected, response.Response.Attempts, true, nil
}

func (p *CallProcessor) recordFailure(failure execution.NodeFailure, order []int) {
	if failure.DescendantsSkipped {
		p.progress.skipped.Add(1)
	}
	p.failures.RecordAt(failure, order)
}
