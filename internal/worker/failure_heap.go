package worker

import "slices"

type failureHeap []failureEntry

func (h failureHeap) Len() int           { return len(h) }
func (h failureHeap) Less(i, j int) bool { return slices.Compare(h[i].order, h[j].order) > 0 }
func (h failureHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *failureHeap) Push(value any)    { *h = append(*h, value.(failureEntry)) }
func (h *failureHeap) Pop() any          { old := *h; v := old[len(old)-1]; *h = old[:len(old)-1]; return v }
