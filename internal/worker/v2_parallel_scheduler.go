package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	"strconv"
	"sync"
)

type v2ParallelScheduler struct {
	runner     *V2Runner
	engine     *Engine
	definition definition.CompiledDefinition
	id         execution.ExecutionID
	owner      string
	generation int64
	checkpoint v2ParallelCheckpoint
	limit      int
	inflight   map[string]bool
	machines   map[string]*v2Machine
	completed  chan v2FetchCompletion
	workers    sync.WaitGroup
	forkError  error
	ctx        context.Context
}

func (s *v2ParallelScheduler) machine(task *v2ParallelTask) *v2Machine {
	m := s.machines[task.Key]
	if m == nil {
		m = &v2Machine{runner: s.runner, engine: s.engine, definition: s.definition, id: s.id, owner: s.owner, generation: s.generation, resultsEnabled: true, parallel: true}
		s.machines[task.Key] = m
	}
	m.checkpoint = &task.Checkpoint
	m.resultNode = task.ResultNode
	m.fork = func(frame v2Frame, commit *execution.V2Commit) bool {
		leaves := 0
		for _, other := range s.checkpoint.Tasks {
			if other.Pending == 0 {
				leaves++
			}
		}
		if task.Pending > 0 && leaves >= s.limit {
			m.blocked = true
			return false
		}
		frame.ErrorStart = 0
		child := &v2ParallelTask{Key: frame.NodeID, Parent: task.Key, Root: task.Root, ResultNode: task.ResultNode, Checkpoint: v2Checkpoint{Version: 2, Stack: []v2Frame{frame}, Report: s.emptyReport()}}
		if frame.Kind == "item" && len(frame.Call) == 1 {
			child.ItemIndex = frame.Index
			child.ResultNode = frame.NodeID
		}
		task.Pending++
		s.checkpoint.Tasks = append(s.checkpoint.Tasks, child)
		// Declare each collection member before admitting its successor. This keeps
		// array tree cursors stable even when descendants finish out of order.
		if frame.Kind == "item" {
			cm := s.machine(child)
			// Input values are immutable. Siblings can share a decoded page while
			// their durable references independently protect its lifetime.
			cm.cachedKey, cm.cachedValue = m.cachedKey, m.cachedValue
			for len(child.Checkpoint.Stack) > 0 {
				before := len(commit.Chunks)
				_, err := cm.step(s.ctx, commit)
				if err != nil {
					s.forkError = err
					break
				}
				if len(commit.Chunks) > before {
					break
				}
			}
		}
		return true
	}
	m.waitChildren = func() bool {
		if task.Pending > 0 {
			m.blocked = true
			return true
		}
		return false
	}
	return m
}

func (s *v2ParallelScheduler) emptyReport() execution.ExecutionErrorReport {
	r := execution.EmptyReport()
	for _, root := range s.definition.Roots {
		r.Roots = append(r.Roots, execution.RootOutcome{Name: root.Name, Status: "not_started"})
	}
	return r
}

func (s *v2ParallelScheduler) references() map[string]bool {
	refs := map[string]bool{}
	for _, task := range s.checkpoint.Tasks {
		for key := range v2ReferencedInputs(task.Checkpoint.Stack) {
			refs[key] = true
		}
	}
	return refs
}

func (s *v2ParallelScheduler) startFetch(ctx context.Context, task *v2ParallelTask) error {
	raw, err := json.Marshal(task.Checkpoint)
	if err != nil {
		return err
	}
	var checkpoint v2Checkpoint
	if err = json.Unmarshal(raw, &checkpoint); err != nil {
		return err
	}
	key, resultNode := task.Key, task.ResultNode
	source := s.machine(task)
	cachedKey, cachedValue := source.cachedKey, source.cachedValue
	s.inflight[key] = true
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		m := v2Machine{runner: s.runner, engine: s.engine, definition: s.definition, id: s.id, owner: s.owner, generation: s.generation, checkpoint: &checkpoint, resultsEnabled: true, parallel: true, resultNode: resultNode}
		m.cachedKey, m.cachedValue = cachedKey, cachedValue
		commit := execution.V2Commit{}
		index := len(checkpoint.Stack) - 1
		_, err := m.step(ctx, &commit)
		if index < 0 {
			err = errors.New("missing fetch frame")
		}
		s.completed <- v2FetchCompletion{Key: key, Checkpoint: checkpoint, Commit: commit, Err: err, CachedKey: m.cachedKey, CachedValue: m.cachedValue}
	}()
	return nil
}

func (s *v2ParallelScheduler) accept(result v2FetchCompletion, commit *execution.V2Commit) error {
	delete(s.inflight, result.Key)
	if result.Err != nil {
		return result.Err
	}
	for _, task := range s.checkpoint.Tasks {
		if task.Key != result.Key {
			continue
		}
		s.addProgress(task.Checkpoint.Progress, result.Checkpoint.Progress)
		task.Checkpoint = result.Checkpoint
		delete(s.machines, task.Key)
		m := s.machine(task)
		m.cachedKey, m.cachedValue = result.CachedKey, result.CachedValue
		commit.Chunks = append(commit.Chunks, result.Commit.Chunks...)
		commit.Inputs = append(commit.Inputs, result.Commit.Inputs...)
		commit.Results = append(commit.Results, result.Commit.Results...)
		commit.ResultErrors = append(commit.ResultErrors, result.Commit.ResultErrors...)
		return nil
	}
	return fmt.Errorf("completion for unknown task %s", result.Key)
}

func (s *v2ParallelScheduler) addProgress(before, after execution.ExecutionProgress) {
	p := &s.checkpoint.Progress
	p.CacheHits += after.CacheHits - before.CacheHits
	p.CoalescedRequests += after.CoalescedRequests - before.CoalescedRequests
	p.Retries += after.Retries - before.Retries
	p.CompletedRootItems += after.CompletedRootItems - before.CompletedRootItems
}

func (s *v2ParallelScheduler) join() {
	for {
		index := -1
		for i, task := range s.checkpoint.Tasks {
			if len(task.Checkpoint.Stack) == 0 && task.Pending == 0 && !s.inflight[task.Key] {
				index = i
				break
			}
		}
		if index < 0 {
			return
		}
		task := s.checkpoint.Tasks[index]
		if task.Parent != "" {
			for _, parent := range s.checkpoint.Tasks {
				if parent.Key == task.Parent {
					parent.Pending--
					s.mergeReport(&parent.Checkpoint.Report, task.Checkpoint.Report)
					break
				}
			}
		} else {
			s.checkpoint.RootDone[task.Root] = true
			s.checkpoint.CompletedReport.Roots[task.Root] = task.Checkpoint.Report.Roots[task.Root]
			s.mergeReport(&s.checkpoint.CompletedReport, task.Checkpoint.Report)
		}
		if task.ItemIndex != nil {
			root := task.Root
			if s.checkpoint.FinishedIndices[root] == nil {
				s.checkpoint.FinishedIndices[root] = map[int64]bool{}
			}
			s.checkpoint.FinishedIndices[root][*task.ItemIndex] = true
			for s.checkpoint.FinishedIndices[root][s.checkpoint.NextIndex[root]] {
				delete(s.checkpoint.FinishedIndices[root], s.checkpoint.NextIndex[root])
				s.checkpoint.NextIndex[root]++
			}
		}
		delete(s.machines, task.Key)
		s.checkpoint.Tasks = append(s.checkpoint.Tasks[:index], s.checkpoint.Tasks[index+1:]...)
	}
}

func (s *v2ParallelScheduler) mergeReport(target *execution.ExecutionErrorReport, source execution.ExecutionErrorReport) {
	target.ErrorSummary.Total += source.ErrorSummary.Total
	for _, failure := range source.Errors {
		if len(target.Errors) >= s.engine.Options.MaxDetails {
			break
		}
		candidate := append(append([]execution.NodeFailure(nil), target.Errors...), failure)
		raw, _ := json.Marshal(candidate)
		if len(raw) > s.engine.Options.MaxReportBytes/2 {
			break
		}
		target.Errors = candidate
	}
	target.ErrorSummary.Returned = uint64(len(target.Errors))
	target.ErrorSummary.Truncated = target.ErrorSummary.Returned < target.ErrorSummary.Total
}

func (s *v2ParallelScheduler) batch(ctx context.Context, commit *execution.V2Commit) (bool, error) {
	s.ctx = ctx
	changed := false
	batchBytes := 0
	batchSteps := 0
	select {
	case result := <-s.completed:
		if err := s.accept(result, commit); err != nil {
			return false, err
		}
		return true, nil
	default:
	}
	tasks := append([]*v2ParallelTask(nil), s.checkpoint.Tasks...)
	start := s.checkpoint.Cursor
	for offset := 0; offset < len(tasks); offset++ {
		position := (start + offset) % len(tasks)
		task := tasks[position]
		s.checkpoint.Cursor = (position + 1) % len(tasks)
		if s.inflight[task.Key] || len(task.Checkpoint.Stack) == 0 {
			continue
		}
		m := s.machine(task)
		before := task.Checkpoint.Progress
		for batchSteps < 1024 && len(task.Checkpoint.Stack) > 0 {
			if err := ctx.Err(); err != nil {
				return false, err
			}
			top := task.Checkpoint.Stack[len(task.Checkpoint.Stack)-1]
			if top.Kind == "call" && top.Phase == "fetch" {
				if len(s.inflight) < s.limit {
					if err := s.startFetch(ctx, task); err != nil {
						return false, err
					}
				}
				break
			}
			m.blocked = false
			batchSteps++
			priorChunks := len(commit.Chunks)
			boundary, err := m.step(ctx, commit)
			for _, chunk := range commit.Chunks[priorChunks:] {
				raw, _ := json.Marshal(chunk)
				batchBytes += len(raw)
			}
			if err != nil {
				return false, err
			}
			if s.forkError != nil {
				return false, s.forkError
			}
			if m.blocked {
				break
			}
			changed = true
			if boundary || len(commit.Chunks) >= 1024 || batchBytes >= 1<<20 {
				break
			}
		}
		s.addProgress(before, task.Checkpoint.Progress)
		if batchSteps >= 1024 || len(commit.Chunks) >= 1024 || batchBytes >= 1<<20 || len(commit.Results) > 0 {
			break
		}
	}
	if changed || batchSteps >= 1024 {
		return true, nil
	}
	if len(s.inflight) > 0 {
		select {
		case result := <-s.completed:
			return true, s.accept(result, commit)
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
	return false, errors.New("concurrent scheduler has no runnable task")
}

func (s *v2ParallelScheduler) sourceFrontier() (int, int64) {
	for root, done := range s.checkpoint.RootDone {
		if !done {
			return root, s.checkpoint.NextIndex[root] - 1
		}
	}
	return len(s.checkpoint.RootDone), -1
}

func (s *v2ParallelScheduler) initialize() {
	s.checkpoint = v2ParallelCheckpoint{Version: 2, Report: s.emptyReport(), CompletedReport: s.emptyReport(), RootDone: make([]bool, len(s.definition.Roots)), NextIndex: make([]int64, len(s.definition.Roots)), FinishedIndices: make([]map[int64]bool, len(s.definition.Roots))}
	for i, root := range s.definition.Roots {
		m := v2Machine{engine: s.engine, id: s.id}
		path := v2Path("", root.Name)
		node := m.nodeID(path)
		task := &v2ParallelTask{Key: "root:" + strconv.Itoa(i), Root: i, Checkpoint: v2Checkpoint{Version: 2, Report: s.emptyReport(), Stack: []v2Frame{{Kind: "call", Call: []int{i}, NodeID: node, Key: root.Name, Path: path}}}}
		if root.Type == "single" {
			task.ResultNode = node
		}
		task.Checkpoint.Report.Roots[i].Status = execution.Running
		s.checkpoint.Tasks = append(s.checkpoint.Tasks, task)
	}
}
