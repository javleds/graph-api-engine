package worker

import execution "github.com/javleds/api-engine/internal/execution/domain"

type v2Checkpoint struct {
	Version      int                            `json:"version"`
	SingleFailed bool                           `json:"singleFailed,omitempty"`
	Stack        []v2Frame                      `json:"stack"`
	Root         int                            `json:"root"`
	Report       execution.ExecutionErrorReport `json:"report"`
	Progress     execution.ExecutionProgress    `json:"progress"`
}
