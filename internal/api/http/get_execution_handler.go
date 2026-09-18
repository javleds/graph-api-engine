package http

import (
	"github.com/javleds/api-engine/internal/api/http/dto"
	"github.com/javleds/api-engine/internal/execution/domain"
	"net/http"
)

func executionLinks(id string) dto.ExecutionLinks {
	base := "/v1/executions/" + id
	return dto.ExecutionLinks{Self: base, Result: base + "/result"}
}
func (s *Server) getExecution(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r, "executions:read")
	if !ok {
		return
	}
	execution, err := s.Reader.Read(r.Context(), principal.OwnerID, domain.ExecutionID(r.PathValue("id")))
	if err != nil {
		presentError(w, err, "")
		return
	}
	report := execution.Report
	if report.Errors == nil {
		report.Errors = []domain.NodeFailure{}
	}
	if report.Roots == nil {
		report.Roots = []domain.RootOutcome{}
	}
	var webhook *dto.WebhookStatusResponse
	if execution.WebhookEnabled {
		webhook = &dto.WebhookStatusResponse{Status: "not_ready"}
	}
	writeJSON(w, 200, dto.ExecutionResponse{ID: execution.ID, Status: execution.Status, CreatedAt: execution.CreatedAt, StartedAt: execution.StartedAt, FinishedAt: execution.FinishedAt, Progress: execution.Progress, Webhook: webhook, Error: execution.Error, Errors: report.Errors, ErrorSummary: report.ErrorSummary, Roots: report.Roots, Links: executionLinks(string(execution.ID))})
}
