package http

import "net/http"

func (s *Server) clearCache(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authorize(w, r, "cache:clear"); !ok {
		return
	}
	if s.ClearCache == nil {
		writeError(w, http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE", "")
		return
	}
	deleted, err := s.ClearCache(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE", "")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"deletedEntries": deleted})
}
