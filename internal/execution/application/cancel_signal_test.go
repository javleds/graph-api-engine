package application

import "github.com/javleds/api-engine/internal/execution/domain"

type cancelSignal struct{ ids []domain.ExecutionID }

func (s *cancelSignal) CancelExecution(id domain.ExecutionID) { s.ids = append(s.ids, id) }
