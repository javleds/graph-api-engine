package http

import (
	"github.com/javleds/api-engine/internal/api/http/dto"
	"github.com/javleds/api-engine/internal/execution/domain"
	"net/http"
)

func (s *Server) getResult(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r, "executions:read")
	if !ok {
		return
	}
	execution, data, err := s.Results.ReadResult(r.Context(), principal.OwnerID, domain.ExecutionID(r.PathValue("id")), s.Clock.Now())
	if err != nil {
		presentError(w, err, "")
		return
	}
	writeJSON(w, 200, dto.ExecutionResultResponse{ID: execution.ID, Status: execution.Status, Result: data, Errors: execution.Report.Errors, ErrorSummary: execution.Report.ErrorSummary, Roots: execution.Report.Roots, Error: execution.Error})
}
