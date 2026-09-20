package http

import (
	"net/http"
	"strconv"

	"github.com/javleds/api-engine/internal/execution/domain"
	notificationports "github.com/javleds/api-engine/internal/notification/application/ports"
)

func (s *Server) getV2Webhooks(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r, "executions:read")
	if !ok {
		return
	}
	after, limit, err := v2PageQuery(r)
	if err != nil {
		presentError(w, err, "")
		return
	}
	id := domain.ExecutionID(r.PathValue("id"))
	execution, err := s.Reader.Read(r.Context(), principal.OwnerID, id)
	if err != nil {
		presentError(w, err, "")
		return
	}
	if !s.matchesVersion(execution) {
		writeError(w, 404, "EXECUTION_NOT_FOUND", "")
		return
	}
	reader, ok := s.WebhookStatus.(notificationports.NodeWebhookStatusReader)
	if !ok {
		writeError(w, 503, "STORAGE_UNAVAILABLE", "")
		return
	}
	page, err := reader.DeliveryPage(r.Context(), principal.OwnerID, id, strconv.FormatInt(after, 10), limit)
	if err != nil {
		presentError(w, err, "")
		return
	}
	writeJSON(w, http.StatusOK, page)
}
