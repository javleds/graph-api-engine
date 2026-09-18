package worker

import (
	"container/heap"
	"encoding/json"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"
)

// BoundedFailureCollector retains the earliest structural failures, not arrivals.
type BoundedFailureCollector struct {
	mu                               sync.Mutex
	total                            uint64
	entries                          failureHeap
	maxDetails, maxBytes, maxMessage int
}

func NewBoundedFailureCollector(details, bytes, message int) *BoundedFailureCollector {
	return &BoundedFailureCollector{maxDetails: details, maxBytes: bytes, maxMessage: message}
}
func (c *BoundedFailureCollector) RecordAt(failure execution.NodeFailure, order []int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.total++
	if c.maxDetails <= 0 {
		return
	}
	if len(failure.Message) > c.maxMessage {
		marker := "…"
		if c.maxMessage < len(marker) {
			marker = strings.Repeat(".", max(0, c.maxMessage))
		}
		failure.Message = failure.Message[:max(0, c.maxMessage-len(marker))]
		for !utf8.ValidString(failure.Message) {
			failure.Message = failure.Message[:len(failure.Message)-1]
		}
		failure.Message += marker
	}
	entry := failureEntry{order: append([]int(nil), order...), failure: failure}
	if len(c.entries) < c.maxDetails {
		heap.Push(&c.entries, entry)
		return
	}
	if slices.Compare(entry.order, c.entries[0].order) < 0 {
		c.entries[0] = entry
		heap.Fix(&c.entries, 0)
	}
}
func (c *BoundedFailureCollector) Record(failure execution.NodeFailure) {
	c.RecordAt(failure, []int{-1})
}
func (c *BoundedFailureCollector) Snapshot() execution.ExecutionErrorReport {
	return c.SnapshotWithRoots(nil)
}
func (c *BoundedFailureCollector) SnapshotWithRoots(roots []execution.RootOutcome) execution.ExecutionErrorReport {
	c.mu.Lock()
	defer c.mu.Unlock()
	entries := append(failureHeap(nil), c.entries...)
	slices.SortFunc(entries, func(a, b failureEntry) int { return slices.Compare(a.order, b.order) })
	report := execution.EmptyReport()
	report.Roots = append(report.Roots, roots...)
	retained := make([]execution.NodeFailure, 0, len(entries))
	used := 0
	for _, entry := range entries {
		encoded, err := json.Marshal(entry.failure)
		if err != nil {
			continue
		}
		extra := len(encoded)
		if len(retained) > 0 {
			extra++
		}
		report.ErrorSummary = execution.ErrorSummary{Total: c.total, Returned: uint64(len(retained) + 1), Truncated: uint64(len(retained)+1) < c.total}
		overhead, _ := json.Marshal(report)
		if len(overhead)+used+extra > c.maxBytes {
			break
		}
		used += extra
		var owned execution.NodeFailure
		_ = json.Unmarshal(encoded, &owned)
		retained = append(retained, owned)
	}
	report.Errors = retained
	report.ErrorSummary = execution.ErrorSummary{Total: c.total, Returned: uint64(len(retained)), Truncated: uint64(len(retained)) < c.total}
	return report
}
