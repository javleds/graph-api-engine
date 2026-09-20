package domain

import execution "github.com/javleds/api-engine/internal/execution/domain"

type NodeEventCursor struct {
	ExecutionID execution.ExecutionID
	OwnerID     string
	After       int64
}
