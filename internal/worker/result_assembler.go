package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	"sync"
)

func (p *CallProcessor) enrich(ctx context.Context, call definition.CompiledCall, n NodeContext, value any) (any, uint64, error) {
	if items, ok := value.([]any); ok {
		if !p.budget.reserveNodes(uint64(len(items))) {
			return []any{}, 0, nil
		}
		slots := make([]any, len(items))
		counts := make([]uint64, len(items))
		fatals := make([]error, len(items))
		var group sync.WaitGroup
		for index, item := range items {
			index, item := index, item
			instance := n
			instance.Path = fmt.Sprintf("%s[%d]", n.Path, index)
			instance.Order = appendOrder(n.Order, index)
			p.branches.Do(&group, func() { slots[index], counts[index], fatals[index] = p.enrichItem(ctx, call, instance, item) })
		}
		group.Wait()
		var count uint64
		for index := range slots {
			count += counts[index]
			if fatals[index] != nil {
				return nil, count, fatals[index]
			}
		}
		if len(n.Values) == 0 {
			p.progress.rootItems.Add(uint64(len(slots)))
		}
		return slots, count, nil
	}
	if !p.budget.reserveNodes(1) {
		return nil, 0, nil
	}
	value, count, err := p.enrichItem(ctx, call, n, value)
	if len(n.Values) == 0 && err == nil {
		p.progress.rootItems.Add(1)
	}
	return value, count, err
}
func (p *CallProcessor) enrichItem(ctx context.Context, call definition.CompiledCall, n NodeContext, value any) (result any, count uint64, fatal error) {
	defer func() {
		if recover() != nil {
			fatal = errors.New("INTERNAL_WORKER_PANIC")
			p.budget.cancel(fatal)
		}
	}()
	if len(call.Children) == 0 {
		raw, err := encodeBounded(value, p.engine.Options.MaxResultBytes)
		if err != nil {
			return nil, 0, err
		}
		if err = p.budget.retainBytes(uint64(len(raw))); err != nil {
			return nil, 0, err
		}
		return value, 0, nil
	}
	var object map[string]any
	switch parent := value.(type) {
	case map[string]any:
		object = make(map[string]any, len(parent))
		for key, item := range parent {
			object[key] = item
		}
	case string, json.Number, bool:
		object = map[string]any{"value": parent}
	default:
		p.failure(call, n, "PARENT_NOT_ENRICHABLE", "assembly", 0)
		return nil, 1, nil
	}
	for _, child := range call.Children {
		if _, exists := object[child.Name]; exists {
			p.failure(call, n, "OUTPUT_FIELD_COLLISION", "assembly", 0)
			return nil, 1, nil
		}
	}
	// Base data and child outputs form a lower bound on final output bytes.
	raw, err := encodeBounded(object, p.engine.Options.MaxResultBytes)
	if err != nil {
		return nil, 0, err
	}
	if err = p.budget.retainBytes(uint64(len(raw))); err != nil {
		return nil, 0, err
	}
	values := make(map[string]any, len(n.Values)+2)
	for name, ancestor := range n.Values {
		values[name] = ancestor
	}
	values[call.Name] = object
	values["parent"] = object
	outcomes := make([]NodeOutcome, len(call.Children))
	fatals := make([]error, len(call.Children))
	var group sync.WaitGroup
	for index, child := range call.Children {
		index, child := index, child
		childContext := n.child(child.Name, index, values)
		p.branches.Do(&group, func() { outcomes[index], fatals[index] = p.Process(ctx, child, childContext) })
	}
	group.Wait()
	// Freeze the context object: only this separate result map receives children.
	resultObject := make(map[string]any, len(object)+len(call.Children))
	for key, item := range object {
		resultObject[key] = item
	}
	for index, child := range call.Children {
		if fatals[index] != nil {
			return nil, count, fatals[index]
		}
		count += outcomes[index].FailureCount
		resultObject[child.Name] = outcomes[index].Value
	}
	return resultObject, count, nil
}
