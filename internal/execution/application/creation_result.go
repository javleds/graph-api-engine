package application

import "github.com/javleds/api-engine/internal/execution/domain"

type CreationResult struct {
	Execution domain.Execution
	Replay    bool
}
