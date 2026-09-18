package worker

import execution "github.com/javleds/api-engine/internal/execution/domain"

type failureEntry struct {
	order   []int
	failure execution.NodeFailure
}
