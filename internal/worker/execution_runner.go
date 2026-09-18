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

type ExecutionRunner struct {
	Store             ports.ExecutionStore
	Finalizer         ports.ExecutionFinalizer
	Engine            *Engine
	InstanceID        string
	ResultRetention   time.Duration
	PrepareCompletion func(context.Context, execution.ExecutionSnapshot, *execution.ExecutionCompletion) error
}

func (r *ExecutionRunner) Run(ctx context.Context, id execution.ExecutionID) error {
	snapshot, current, err := r.Store.Load(ctx, id)
	if err != nil {
		return err
	}
	if current.Status != execution.Pending {
		return nil
	}
	claimed, err := r.Store.Claim(ctx, id, r.InstanceID, current.Version, time.Now().UTC())
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	var compiled definition.CompiledDefinition
	decoder := json.NewDecoder(bytes.NewReader(snapshot.Definition))
	decoder.UseNumber()
	if err = decoder.Decode(&compiled); err != nil || compiled.Version != 1 {
		return r.finishFailure(ctx, id, current.Version+1, "UNSUPPORTED_VERSION")
	}
	completion := r.Engine.Execute(ctx, compiled, snapshot.OwnerID, id)
	completion.ExpectedVersion = current.Version + 1
	retention := r.ResultRetention
	if retention <= 0 {
		retention = 7 * 24 * time.Hour
	}
	completion.ResultExpiresAt = completion.FinishedAt.Add(retention)
	timeout := r.Engine.Options.defaults().PersistenceTimeout
	persist, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	if r.PrepareCompletion != nil {
		if err = r.PrepareCompletion(persist, snapshot, &completion); err != nil {
			return err
		}
	}
	return r.Finalizer.Finish(persist, completion)
}
func (r *ExecutionRunner) finishFailure(ctx context.Context, id execution.ExecutionID, version int64, code string) error {
	if r.Finalizer == nil {
		return errors.New("execution finalizer is required")
	}
	persist, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.Engine.Options.defaults().PersistenceTimeout)
	defer cancel()
	return r.Finalizer.Finish(persist, execution.ExecutionCompletion{ID: id, ExpectedVersion: version, Status: execution.Failed, Error: &execution.ExecutionFailure{Code: code, Message: "La definición persistida no es compatible."}, Report: execution.EmptyReport(), FinishedAt: time.Now().UTC()})
}
