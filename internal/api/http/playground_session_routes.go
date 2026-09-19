package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	security "github.com/javleds/api-engine/internal/security/infrastructure"
)

const playgroundSessionCookie = "api_engine_playground_session"

type playgroundLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) registerPlaygroundSession(mux *http.ServeMux) {
	if s.PlaygroundSession == nil {
		return
	}
	mux.HandleFunc("POST /playground/api/session", s.loginPlayground)
	mux.HandleFunc("GET /playground/api/session", s.playgroundSessionStatus)
	mux.HandleFunc("DELETE /playground/api/session", s.logoutPlayground)
}

func (s *Server) loginPlayground(w http.ResponseWriter, r *http.Request) {
	var input playgroundLoginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_LOGIN", "")
		return
	}
	token, err := s.PlaygroundSession.Login(input.Username, input.Password)
	if err != nil {
		if errors.Is(err, security.ErrInvalidPlaygroundCredentials) {
			writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "LOGIN_UNAVAILABLE", "")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: playgroundSessionCookie, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: s.PlaygroundCookieSecure, MaxAge: int((8 * time.Hour).Seconds())})
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
}

func (s *Server) playgroundSessionStatus(w http.ResponseWriter, r *http.Request) {
	local := *s
	local.Auth = s.PlaygroundSession
	local.PlaygroundCookieName = playgroundSessionCookie
	if _, ok := local.authorize(w, r, "executions:read"); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
}

func (s *Server) logoutPlayground(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: playgroundSessionCookie, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: s.PlaygroundCookieSecure, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}
