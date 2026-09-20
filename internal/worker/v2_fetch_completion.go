package worker

import execution "github.com/javleds/api-engine/internal/execution/domain"

type v2FetchCompletion struct {
	CachedKey   string
	CachedValue any
	Key         string
	Checkpoint  v2Checkpoint
	Commit      execution.V2Commit
	Err         error
}
