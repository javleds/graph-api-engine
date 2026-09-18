package worker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	"sync"
)

func (p *CallProcessor) paginate(ctx context.Context, call definition.CompiledCall, n NodeContext) (NodeOutcome, error) {
	outcome := emptyOutcome(call)
	output := make([]any, 0)
	page := call.Pagination.Start
	expected := int64(0)
	accepted := int64(0)
	var previous [32]byte
	if call.Pagination.PageSize != "" {
		text, _ := definition.ScalarString(call.Request.Query[call.Pagination.PageSize])
		_, _ = fmt.Sscan(text, &expected)
	}
	for ctx.Err() == nil {
		current := n
		current.Page = &page
		current.Order = appendOrder(n.Order, int(accepted))
		selected, attempts, ok, fatal := p.fetch(ctx, call, current)
		if fatal != nil {
			return outcome, fatal
		}
		if !ok {
			outcome.Incomplete = true
			if ctx.Err() == nil {
				outcome.FailureCount++
			}
			break
		}
		items := selected.([]any)
		size := int64(len(items))
		if expected == 0 && size > 0 {
			expected = size
		}
		if expected > 0 && size > expected {
			p.failure(call, current, "PAGINATION_SIZE_INCONSISTENT", "pagination", attempts)
			outcome.FailureCount++
			outcome.Incomplete = true
			break
		}
		raw, _ := json.Marshal(selected)
		digest := sha256.Sum256(raw)
		if p.engine.Options.DetectRepeatedPages && accepted > 0 && size == expected && size > 0 && digest == previous {
			p.failure(call, current, "PAGINATION_NO_PROGRESS", "pagination", attempts)
			outcome.FailureCount++
			outcome.Incomplete = true
			break
		}
		previous = digest
		accepted++
		outcome.OwnDataAvailable = true
		// Instance indexes are absolute across accepted pages.
		if !p.budget.reserveNodes(uint64(len(items))) {
			outcome.Incomplete = true
			break
		}
		enriched := make([]any, len(items))
		counts := make([]uint64, len(items))
		fatals := make([]error, len(items))
		var group sync.WaitGroup
		offset := len(output)
		for index, item := range items {
			index, item := index, item
			instance := current
			instance.Path = fmt.Sprintf("%s[%d]", n.Path, offset+index)
			instance.Order = appendOrder(current.Order, index)
			p.branches.Do(&group, func() { enriched[index], counts[index], fatals[index] = p.enrichItem(ctx, call, instance, item) })
		}
		group.Wait()
		for index := range items {
			if fatals[index] != nil {
				return outcome, fatals[index]
			}
			outcome.FailureCount += counts[index]
		}
		output = append(output, enriched...)
		if len(n.Values) == 0 {
			p.progress.rootItems.Add(uint64(len(enriched)))
		}
		if size == 0 || size < expected || (!call.Pagination.Pages.All && accepted >= call.Pagination.Pages.Count) {
			break
		}
		if accepted >= int64(p.engine.Options.MaxPagesPerCall) {
			p.failure(call, current, "PAGINATION_LIMIT_REACHED", "pagination", 0)
			outcome.FailureCount++
			outcome.Incomplete = true
			break
		}
		page++
	}
	outcome.Incomplete = outcome.Incomplete || outcome.FailureCount > 0 || ctx.Err() != nil
	raw, err := encodeBounded(output, p.engine.Options.MaxResultBytes)
	if err != nil {
		return outcome, err
	}
	outcome.Value = raw
	return outcome, nil
}
