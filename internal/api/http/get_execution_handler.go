package http

import (
	"github.com/javleds/api-engine/internal/api/http/dto"
	"github.com/javleds/api-engine/internal/execution/domain"
	notificationports "github.com/javleds/api-engine/internal/notification/application/ports"
	notification "github.com/javleds/api-engine/internal/notification/domain"
	"net/http"
)

func (s *Server) executionLinks(id string) dto.ExecutionLinks {
	prefix := s.route.executionBasePath
	base := prefix + "/" + id
	if s.apiVersion() == 2 {
		return dto.ExecutionLinks{Self: base, Chunks: base + "/chunks", Tree: base + "/tree", Export: base + "/export", Results: base + "/results"}
	}
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
	if !s.matchesVersion(execution) {
		writeError(w, 404, "EXECUTION_NOT_FOUND", "")
		return
	}
	var state *domain.V2State
	if s.apiVersion() == 2 {
		v, err := s.V2Store.ReadV2State(r.Context(), execution.ID)
		if err != nil {
			presentError(w, err, "")
			return
		}
		state = &v
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
		if s.WebhookStatus != nil {
			status, err := s.WebhookStatus.Status(r.Context(), principal.OwnerID, execution.ID)
			if err != nil {
				presentError(w, err, "")
				return
			}
			if status != nil {
				webhook = &dto.WebhookStatusResponse{Status: status.Status, Attempts: status.Attempts}
			}
		}
	}
	var nodeWebhooks *notification.NodeWebhookSummary
	if state != nil && state.ResultsEnabled && execution.WebhookEnabled {
		if reader, ok := s.WebhookStatus.(notificationports.NodeWebhookStatusReader); ok {
			summary, err := reader.NodeSummary(r.Context(), principal.OwnerID, execution.ID)
			if err != nil {
				presentError(w, err, "")
				return
			}
			nodeWebhooks = &summary
		}
	}
	writeJSON(w, 200, dto.ExecutionResponse{NodeWebhookSummary: nodeWebhooks, V2: state, ID: execution.ID, Status: execution.Status, CreatedAt: execution.CreatedAt, StartedAt: execution.StartedAt, FinishedAt: execution.FinishedAt, Progress: execution.Progress, Webhook: webhook, Error: execution.Error, Errors: report.Errors, ErrorSummary: report.ErrorSummary, Roots: report.Roots, Links: s.executionLinks(string(execution.ID))})
}
