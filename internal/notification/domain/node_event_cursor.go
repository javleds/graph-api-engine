package domain

import execution "github.com/javleds/api-engine/internal/execution/domain"

type NodeEventCursor struct {
	Source      bool
	ExecutionID execution.ExecutionID
	OwnerID     string
	After       int64
}
