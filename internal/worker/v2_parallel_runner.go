package worker

import (
	"context"
	"encoding/json"
	"errors"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	"strconv"
	"time"
)

func (r *V2Runner) runConcurrent(parent context.Context, snapshot execution.ExecutionSnapshot, current execution.Execution, state execution.V2State, compiled definition.CompiledDefinition) error {
	engine := *r.Engine
	engine.Options = engine.Options.defaults()
	ctx, cancel := context.WithDeadline(parent, state.DeadlineAt)
	s := &v2ParallelScheduler{runner: r, engine: &engine, definition: compiled, id: snapshot.ID, owner: snapshot.OwnerID, generation: state.Generation, limit: engine.Options.MaxBranchGoroutines, inflight: map[string]bool{}, machines: map[string]*v2Machine{}}
	s.completed = make(chan v2FetchCompletion, s.limit)
	defer func() { cancel(); s.workers.Wait() }()
	s.initialize()
	payload, err := json.Marshal(s.checkpoint)
	if err != nil {
		return err
	}
	if err = r.V2Store.SeedV2Tasks(parent, s.id, s.owner, s.generation, []execution.V2Task{{Key: "coordinator", Payload: payload}}); err != nil {
		return err
	}
	task, err := r.V2Store.ReadV2Task(parent, s.id, "coordinator")
	if err != nil {
		return err
	}
	if err = json.Unmarshal(task.Payload, &s.checkpoint); err != nil {
		return err
	}
	limits := r.Limits
	if limits.MaxPublishedBytes <= 0 {
		limits = execution.DefaultV2Limits()
	}
	var fatal *execution.ExecutionFailure
	if s.checkpoint.Version != 2 || compiled.Version != 1 {
		fatal = &execution.ExecutionFailure{Code: "UNSUPPORTED_CHECKPOINT_VERSION", Message: "El checkpoint persistido no es compatible."}
	}
	for !task.Done && fatal == nil {
		if parent.Err() != nil {
			return parent.Err()
		}
		before := s.references()
		commit := execution.V2Commit{ID: s.id, OwnerID: s.owner, Generation: s.generation, Task: task, Limits: limits}
		_, err = s.batch(ctx, &commit)
		if err == nil {
			s.join()
			if int64(len(s.checkpoint.Tasks)) > limits.MaxPendingTasks {
				err = execution.ErrV2Limit
			}
		}
		if err == nil {
			after := s.references()
			for key := range before {
				if !after[key] {
					commit.DeleteInputs = append(commit.DeleteInputs, key)
				}
			}
			for key, m := range s.machines {
				if !after[m.cachedKey] || len(s.inflight) > 0 && m.waitChildren != nil && m.waitChildren() {
					m.cachedKey = ""
					m.cachedValue = nil
					s.machines[key] = m
				}
			}
			state, err = r.V2Store.ReadV2State(ctx, s.id)
			if err == nil {
				report := s.report()
				s.checkpoint.Report = report
				s.checkpoint.Progress.LogicalRequests = uint64(state.LogicalRequests + commit.LogicalReservations)
				s.checkpoint.Progress.HTTPAttempts = uint64(state.HTTPAttempts)
				s.checkpoint.Progress.NodeFailures = report.ErrorSummary.Total
				s.updateRootProgress(report)
				commit.Progress = &s.checkpoint.Progress
				root, index := s.sourceFrontier()
				commit.SourceRoot = &root
				commit.SourceIndex = index
				commit.Task.Payload, err = json.Marshal(s.checkpoint)
				commit.Task.Done = len(s.checkpoint.Tasks) == 0
			}
			if err == nil {
				err = r.V2Store.CommitV2Task(ctx, commit)
			}
		}
		if err != nil {
			if parent.Err() != nil {
				return parent.Err()
			}
			code := ""
			switch {
			case errors.Is(err, context.DeadlineExceeded):
				code = "EXECUTION_TIMEOUT"
			case errors.Is(err, execution.ErrV2Limit):
				code = "EXECUTION_BUDGET_EXCEEDED"
			case errors.Is(err, execution.ErrV2ValueTooLarge):
				code = "RESULT_VALUE_TOO_LARGE"
			case err.Error() == "INTERNAL_WORKER_PANIC" || err.Error() == "INTERNAL_WORKER_ERROR":
				code = err.Error()
			}
			if code == "" {
				return err
			}
			fatal = &execution.ExecutionFailure{Code: code, Message: failureMessage(code)}
			break
		}
		task = commit.Task
		task.Revision++
		sequence, _ := strconv.ParseInt(state.LastSequence, 10, 64)
		for _, m := range s.machines {
			m.sequence = sequence + int64(len(commit.Chunks))
		}
	}
	cancel()
	s.workers.Wait()
	if parent.Err() != nil {
		return parent.Err()
	}
	if fatal != nil {
		saved, readErr := r.V2Store.ReadV2Task(parent, s.id, "coordinator")
		if readErr != nil {
			return readErr
		}
		if err = json.Unmarshal(saved.Payload, &s.checkpoint); err != nil {
			return err
		}
	}
	report := s.report()
	if fatal != nil {
		for i := range report.Roots {
			if !s.checkpoint.RootDone[i] {
				report.Roots[i].Status = execution.Failed
				if report.Roots[i].DataAvailable {
					report.Roots[i].Status = execution.CompletedWithErrors
				}
			}
		}
	}
	state, err = r.V2Store.ReadV2State(parent, s.id)
	if err != nil {
		return err
	}
	s.checkpoint.Progress.LogicalRequests = uint64(state.LogicalRequests)
	s.checkpoint.Progress.HTTPAttempts = uint64(state.HTTPAttempts)
	s.checkpoint.Progress.NodeFailures = report.ErrorSummary.Total
	s.updateRootProgress(report)
	status, failure := execution.ResolveOutcome(report.Roots, report.ErrorSummary.Total, fatal)
	finished := time.Now().UTC()
	retention := r.ResultRetention
	if retention <= 0 {
		retention = 7 * 24 * time.Hour
	}
	completion := execution.ExecutionCompletion{ID: s.id, APIVersion: 2, ExpectedVersion: current.Version, Status: status, Error: failure, Report: report, Progress: s.checkpoint.Progress, FinishedAt: finished, ResultExpiresAt: finished.Add(retention)}
	persist, stop := context.WithTimeout(context.WithoutCancel(parent), engine.Options.PersistenceTimeout)
	defer stop()
	if r.PrepareCompletion != nil {
		if err = r.PrepareCompletion(persist, snapshot, &completion); err != nil {
			return err
		}
	}
	return r.Finalizer.Finish(persist, completion)
}

func (s *v2ParallelScheduler) report() execution.ExecutionErrorReport {
	report := s.emptyReport()
	copy(report.Roots, s.checkpoint.CompletedReport.Roots)
	s.mergeReport(&report, s.checkpoint.CompletedReport)
	for _, task := range s.checkpoint.Tasks {
		s.mergeReport(&report, task.Checkpoint.Report)
		if task.Parent == "" {
			report.Roots[task.Root] = task.Checkpoint.Report.Roots[task.Root]
		}
	}
	return report
}

func (s *v2ParallelScheduler) updateRootProgress(report execution.ExecutionErrorReport) {
	p := &s.checkpoint.Progress
	p.SuccessfulRootCalls = 0
	p.PartialRootCalls = 0
	p.FailedRootCalls = 0
	for _, root := range report.Roots {
		switch root.Status {
		case execution.Completed:
			p.SuccessfulRootCalls++
		case execution.CompletedWithErrors:
			p.PartialRootCalls++
		case execution.Failed:
			p.FailedRootCalls++
		}
	}
}
