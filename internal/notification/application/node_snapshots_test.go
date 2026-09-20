package application

import (
	"context"
	executionports "github.com/javleds/api-engine/internal/execution/application/ports"
	execution "github.com/javleds/api-engine/internal/execution/domain"
)

type nodeSnapshots struct{ executionports.ExecutionStore }

func (nodeSnapshots) Load(context.Context, execution.ExecutionID) (execution.ExecutionSnapshot, execution.Execution, error) {
	return execution.ExecutionSnapshot{Definition: []byte(`{"webhook":{"url":"https://receiver.example","method":"POST","payload":"status"}}`)}, execution.Execution{}, nil
}
