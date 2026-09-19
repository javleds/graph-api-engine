package worker

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/javleds/api-engine/internal/execution/application/ports"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	"time"
)

type v2MemoryStore struct {
	ports.V2Store
	task        execution.V2Task
	inputs      map[string]json.RawMessage
	chunks      []execution.V2Chunk
	state       execution.V2State
	commits     int
	crashAt     int
	afterCommit bool
}

func (s *v2MemoryStore) ReadV2State(context.Context, execution.ExecutionID) (execution.V2State, error) {
	return s.state, nil
}
func (s *v2MemoryStore) ReadV2Task(context.Context, execution.ExecutionID, string) (execution.V2Task, error) {
	return s.task, nil
}
func (s *v2MemoryStore) SeedV2Tasks(_ context.Context, _ execution.ExecutionID, _ string, _ int64, tasks []execution.V2Task) error {
	if s.task.Key == "" {
		s.task = tasks[0]
	}
	return nil
}
func (s *v2MemoryStore) ReadV2Input(_ context.Context, _ execution.ExecutionID, key string) (json.RawMessage, error) {
	v, ok := s.inputs[key]
	if !ok {
		return nil, errors.New("missing input")
	}
	return v, nil
}
func (s *v2MemoryStore) ReserveV2Budget(_ context.Context, _ execution.ExecutionID, _ int64, kind string, maximum int64) error {
	counter := &s.state.LogicalRequests
	if kind == "attempt" {
		counter = &s.state.HTTPAttempts
	}
	if *counter >= maximum {
		return execution.ErrV2Limit
	}
	*counter++
	return nil
}
func (s *v2MemoryStore) CommitV2Task(_ context.Context, c execution.V2Commit) error {
	s.commits++
	if s.commits == s.crashAt && !s.afterCommit {
		return errors.New("simulated storage interruption")
	}
	if c.Task.Revision != s.task.Revision {
		return errors.New("revision conflict")
	}
	s.task = c.Task
	s.task.Revision++
	if s.inputs == nil {
		s.inputs = map[string]json.RawMessage{}
	}
	for _, input := range c.Inputs {
		s.inputs[input.Key] = append(json.RawMessage(nil), input.Payload...)
	}
	for _, key := range c.DeleteInputs {
		delete(s.inputs, key)
	}
	s.chunks = append(s.chunks, c.Chunks...)
	if s.commits == s.crashAt {
		return errors.New("simulated ambiguous commit")
	}
	return nil
}
func newV2MemoryStore() *v2MemoryStore {
	return &v2MemoryStore{state: execution.V2State{CheckpointVersion: 1, DeadlineAt: time.Now().Add(time.Minute), Generation: 1}}
}
