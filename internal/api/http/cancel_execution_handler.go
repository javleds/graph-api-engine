package http

import (
	"github.com/javleds/api-engine/internal/api/http/dto"
	"github.com/javleds/api-engine/internal/execution/domain"
	"net/http"
)

func (s *Server) cancelExecution(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r, "executions:create")
	if !ok {
		return
	}
	if !principal.Can("executions:read") {
		writeError(w, 403, "FORBIDDEN", "")
		return
	}
	if s.Cancel == nil {
		writeError(w, 503, "STORAGE_UNAVAILABLE", "")
		return
	}
	execution, err := s.Cancel.Execute(r.Context(), principal.OwnerID, domain.ExecutionID(r.PathValue("id")), s.apiVersion())
	if err != nil {
		presentError(w, err, "")
		return
	}
	writeJSON(w, http.StatusOK, dto.CancelExecutionResponse{ID: execution.ID, Status: execution.Status, Error: execution.Error, Links: s.executionLinks(string(execution.ID))})
}
