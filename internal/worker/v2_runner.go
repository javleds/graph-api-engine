package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	"github.com/javleds/api-engine/internal/execution/application/ports"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	"time"
)

// V2Runner checkpoints a bounded continuation stack instead of retaining results.
// A canceled service context deliberately leaves the job resumable.
type V2Runner struct {
	Store             ports.ExecutionStore
	V2Store           ports.V2Store
	Finalizer         ports.ExecutionFinalizer
	Engine            *Engine
	InstanceID        string
	ResultRetention   time.Duration
	Limits            execution.V2Limits
	PrepareCompletion func(context.Context, execution.ExecutionSnapshot, *execution.ExecutionCompletion) error
}

func (r *V2Runner) Run(parent context.Context, id execution.ExecutionID) error {
	snapshot, current, err := r.Store.Load(parent, id)
	if err != nil {
		return err
	}
	if current.Status != execution.Pending {
		return nil
	}
	if current.Status == execution.Pending {
		claimed, claimErr := r.Store.Claim(parent, id, r.InstanceID, current.Version, time.Now().UTC())
		if claimErr != nil {
			return claimErr
		}
		if !claimed {
			return nil
		}
		current.Version++
	}
	state, err := r.V2Store.ReadV2State(parent, id)
	if err != nil {
		return err
	}
	var compiled definition.CompiledDefinition
	decoder := json.NewDecoder(bytes.NewReader(snapshot.Definition))
	decoder.UseNumber()
	if err = decoder.Decode(&compiled); err != nil {
		return err
	}
	checkpoint := v2Checkpoint{Version: 1, Report: execution.EmptyReport()}
	for _, root := range compiled.Roots {
		checkpoint.Report.Roots = append(checkpoint.Report.Roots, execution.RootOutcome{Name: root.Name, Status: "not_started"})
	}
	payload, _ := json.Marshal(checkpoint)
	if err = r.V2Store.SeedV2Tasks(parent, id, snapshot.OwnerID, state.Generation, []execution.V2Task{{Key: "coordinator", Payload: payload}}); err != nil {
		return err
	}
	task, err := r.V2Store.ReadV2Task(parent, id, "coordinator")
	if err != nil {
		return err
	}
	if err = json.Unmarshal(task.Payload, &checkpoint); err != nil {
		return err
	}
	var fatal *execution.ExecutionFailure
	if checkpoint.Version != 1 || state.CheckpointVersion != 1 || compiled.Version != 1 {
		fatal = &execution.ExecutionFailure{Code: "UNSUPPORTED_CHECKPOINT_VERSION", Message: "El checkpoint persistido no es compatible."}
	}
	configured := *r.Engine
	configured.Options = r.Engine.Options.defaults()
	limits := r.Limits
	if limits.MaxPublishedBytes <= 0 {
		limits = execution.DefaultV2Limits()
	}
	ctx, cancel := context.WithDeadline(parent, state.DeadlineAt)
	defer cancel()
	machine := v2Machine{runner: r, engine: &configured, definition: compiled, id: id, owner: snapshot.OwnerID, generation: state.Generation, checkpoint: &checkpoint}
	for !task.Done && fatal == nil {
		if parent.Err() != nil {
			return parent.Err()
		}
		if ctx.Err() != nil {
			fatal = &execution.ExecutionFailure{Code: "EXECUTION_TIMEOUT", Message: failureMessage("EXECUTION_TIMEOUT")}
			break
		}
		referencedBefore := v2ReferencedInputs(checkpoint.Stack)
		commit := execution.V2Commit{ID: id, OwnerID: snapshot.OwnerID, Generation: state.Generation, Task: task, Limits: limits}
		batchBytes := 0
		for steps := 0; steps < 128; steps++ {
			priorChunks := len(commit.Chunks)
			boundary, stepErr := machine.step(ctx, &commit)
			if stepErr != nil {
				err = stepErr
				break
			}
			for _, chunk := range commit.Chunks[priorChunks:] {
				raw, _ := json.Marshal(chunk)
				batchBytes += len(raw)
			}
			if boundary || batchBytes >= 1<<20 || len(commit.Chunks) >= 128 || checkpoint.Root >= len(compiled.Roots) && len(checkpoint.Stack) == 0 {
				break
			}
		}
		if err != nil {
			if parent.Err() != nil {
				return parent.Err()
			}
			if errors.Is(err, context.DeadlineExceeded) {
				fatal = &execution.ExecutionFailure{Code: "EXECUTION_TIMEOUT", Message: failureMessage("EXECUTION_TIMEOUT")}
				break
			}
			if err.Error() == "INTERNAL_WORKER_PANIC" || err.Error() == "INTERNAL_WORKER_ERROR" {
				fatal = &execution.ExecutionFailure{Code: err.Error(), Message: "No se pudo producir un resultado confiable."}
				break
			}
			if errors.Is(err, execution.ErrV2Limit) {
				fatal = &execution.ExecutionFailure{Code: "EXECUTION_BUDGET_EXCEEDED", Message: failureMessage("EXECUTION_BUDGET_EXCEEDED")}
				break
			}
			return err
		}
		referencedAfter := v2ReferencedInputs(checkpoint.Stack)
		for key := range referencedBefore {
			if !referencedAfter[key] {
				commit.DeleteInputs = append(commit.DeleteInputs, key)
			}
		}
		budgetState, readErr := r.V2Store.ReadV2State(ctx, id)
		if readErr != nil {
			if errors.Is(readErr, context.DeadlineExceeded) && parent.Err() == nil {
				fatal = &execution.ExecutionFailure{Code: "EXECUTION_TIMEOUT", Message: failureMessage("EXECUTION_TIMEOUT")}
				break
			}
			return readErr
		}
		checkpoint.Progress.LogicalRequests = uint64(budgetState.LogicalRequests)
		checkpoint.Progress.HTTPAttempts = uint64(budgetState.HTTPAttempts)
		checkpoint.Progress.NodeFailures = checkpoint.Report.ErrorSummary.Total
		commit.Progress = &checkpoint.Progress
		commit.Task.Payload, err = json.Marshal(checkpoint)
		if err != nil {
			return err
		}
		commit.Task.Done = checkpoint.Root >= len(compiled.Roots) && len(checkpoint.Stack) == 0
		if err = r.V2Store.CommitV2Task(ctx, commit); err != nil {
			if parent.Err() != nil {
				return parent.Err()
			}
			if errors.Is(err, context.DeadlineExceeded) {
				fatal = &execution.ExecutionFailure{Code: "EXECUTION_TIMEOUT", Message: failureMessage("EXECUTION_TIMEOUT")}
				break
			}
			code := ""
			if errors.Is(err, execution.ErrV2ValueTooLarge) {
				code = "RESULT_VALUE_TOO_LARGE"
			}
			if errors.Is(err, execution.ErrV2Limit) {
				code = "EXECUTION_BUDGET_EXCEEDED"
			}
			if code == "" {
				return err
			}
			fatal = &execution.ExecutionFailure{Code: code, Message: "La ejecución alcanzó un límite de publicación."}
			break
		}
		task = commit.Task
		task.Revision++
	}
	if parent.Err() != nil {
		return parent.Err()
	}
	// Reload only committed diagnostics when a publication transaction failed.
	if fatal != nil {
		saved, readErr := r.V2Store.ReadV2Task(parent, id, "coordinator")
		if readErr != nil {
			return readErr
		}
		if err = json.Unmarshal(saved.Payload, &checkpoint); err != nil {
			return err
		}
		for i := range checkpoint.Report.Roots {
			root := &checkpoint.Report.Roots[i]
			if root.Status == "not_started" && i == checkpoint.Root {
				root.Status = execution.Failed
			}
			if root.Status == execution.Running {
				if root.DataAvailable {
					root.Status = execution.CompletedWithErrors
				} else {
					root.Status = execution.Failed
				}
			}
		}
	}
	state, err = r.V2Store.ReadV2State(parent, id)
	if err != nil {
		return err
	}
	checkpoint.Progress.LogicalRequests = uint64(state.LogicalRequests)
	checkpoint.Progress.HTTPAttempts = uint64(state.HTTPAttempts)
	checkpoint.Progress.NodeFailures = checkpoint.Report.ErrorSummary.Total
	status, failure := execution.ResolveOutcome(checkpoint.Report.Roots, checkpoint.Report.ErrorSummary.Total, fatal)
	finished := time.Now().UTC()
	retention := r.ResultRetention
	if retention <= 0 {
		retention = 7 * 24 * time.Hour
	}
	completion := execution.ExecutionCompletion{ID: id, APIVersion: 2, ExpectedVersion: current.Version, Status: status, Error: failure, Report: checkpoint.Report, Progress: checkpoint.Progress, FinishedAt: finished, ResultExpiresAt: finished.Add(retention)}
	persist, cancelPersist := context.WithTimeout(context.WithoutCancel(parent), configured.Options.PersistenceTimeout)
	defer cancelPersist()
	if r.PrepareCompletion != nil {
		if err = r.PrepareCompletion(persist, snapshot, &completion); err != nil {
			return err
		}
	}
	return r.Finalizer.Finish(persist, completion)
}

func v2ReferencedInputs(stack []v2Frame) map[string]bool {
	keys := map[string]bool{}
	for _, frame := range stack {
		if frame.Ref.Key != "" {
			keys[frame.Ref.Key] = true
		}
		for _, ref := range frame.Ancestors {
			keys[ref.Key] = true
		}
	}
	return keys
}
