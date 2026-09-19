package http

import (
	"github.com/javleds/api-engine/internal/execution/domain"
	"net/http"
	"strconv"
)

func v2PageQuery(r *http.Request) (int64, int, error) {
	query := r.URL.Query()
	after := int64(0)
	if values, exists := query["after"]; exists {
		if len(values) != 1 {
			return 0, 0, domain.ErrV2Cursor
		}
		value, err := strconv.ParseInt(values[0], 10, 64)
		if err != nil || value < 0 || strconv.FormatInt(value, 10) != values[0] {
			return 0, 0, domain.ErrV2Cursor
		}
		after = value
	}
	limit := 100
	if values, exists := query["limit"]; exists {
		if len(values) != 1 {
			return 0, 0, domain.ErrV2Cursor
		}
		value, err := strconv.Atoi(values[0])
		if err != nil || value < 1 || value > 100 {
			return 0, 0, domain.ErrV2Cursor
		}
		limit = value
	}
	return after, limit, nil
}
func (s *Server) getV2Chunks(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r, "executions:read")
	if !ok {
		return
	}
	after, limit, err := v2PageQuery(r)
	if err != nil {
		presentError(w, err, "")
		return
	}
	page, err := s.V2Store.ReadV2Chunks(r.Context(), principal.OwnerID, domain.ExecutionID(r.PathValue("id")), after, limit, 1<<20, s.Clock.Now())
	if err != nil {
		presentError(w, err, "")
		return
	}
	writeJSON(w, 200, page)
}
func (s *Server) getV2Tree(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r, "executions:read")
	if !ok {
		return
	}
	after, limit, err := v2PageQuery(r)
	if err != nil {
		presentError(w, err, "")
		return
	}
	page, err := s.V2Store.ReadV2Tree(r.Context(), principal.OwnerID, domain.ExecutionID(r.PathValue("id")), r.URL.Query().Get("parentId"), after, limit, s.Clock.Now())
	if err != nil {
		presentError(w, err, "")
		return
	}
	writeJSON(w, 200, page)
}
