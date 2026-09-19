package http

import (
	securitydomain "github.com/javleds/api-engine/internal/security/domain"
	"net/http"
	"strings"
)

func (s *Server) authorize(w http.ResponseWriter, r *http.Request, permission string) (securitydomain.Principal, bool) {
	token := s.credential(r)
	principal, err := s.Auth.Authenticate(r.Context(), token)
	if err != nil {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, 401, "UNAUTHENTICATED", "")
		return principal, false
	}
	if !principal.Can(permission) {
		writeError(w, 403, "FORBIDDEN", "")
		return principal, false
	}
	return principal, true
}

func (s *Server) credential(r *http.Request) string {
	if s.PlaygroundCookieName != "" {
		cookie, err := r.Cookie(s.PlaygroundCookieName)
		if err == nil {
			return cookie.Value
		}
		return ""
	}
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}
	return ""
}
