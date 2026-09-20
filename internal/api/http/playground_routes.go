package http

import (
	"io"
	"net/http"

	"github.com/javleds/api-engine/internal/api/http/playground"
)

func (s *Server) registerPlayground(mux *http.ServeMux) {
	playground.Register(mux, s.authorizeWebPage)
	mux.HandleFunc("POST /playground/api/webhook", s.playgroundWebhook)
	s.registerPlaygroundSession(mux)
	s.registerPlaygroundConfiguration(mux)
	if s.PlaygroundAuth == nil {
		return
	}
	local := *s
	local.Auth = s.PlaygroundAuth
	if s.PlaygroundSession != nil {
		local.Auth = s.PlaygroundSession
		local.PlaygroundCookieName = playgroundSessionCookie
	}
	local.ExecutionBasePath = "/playground/api/executions"
	protection := http.NewCrossOriginProtection()
	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "")
	}))
	mux.Handle("POST /playground/api/validate", protection.Handler(http.HandlerFunc(local.validateRequest)))
	mux.Handle("POST /playground/api/executions", protection.Handler(http.HandlerFunc(local.createExecution)))
	mux.Handle("POST /playground/api/executions/{id}/cancel", protection.Handler(http.HandlerFunc(local.cancelExecution)))
	mux.HandleFunc("GET /playground/api/executions/{id}", local.getExecution)
	mux.HandleFunc("GET /playground/api/executions/{id}/result", local.getResult)
	v2 := local
	v2.APIVersion = 2
	v2.ExecutionBasePath = "/playground/api/v2/executions"
	v2.registerV2(mux, protection)
	mux.Handle("POST /playground/api/v2/validate", protection.Handler(http.HandlerFunc(v2.validateRequest)))
}

// playgroundWebhook is a deliberately small local receiver used by the
// authenticated examples. It acknowledges demo deliveries without exposing
// them as an application webhook integration.
func (s *Server) playgroundWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-API-Engine-Playground") != "demo" {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "")
		return
	}
	defer r.Body.Close()
	_, _ = io.Copy(io.Discard, http.MaxBytesReader(w, r.Body, 1<<20))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) authorizeWebPage(w http.ResponseWriter, r *http.Request) bool {
	if s.PlaygroundSession == nil {
		return true
	}
	cookie, err := r.Cookie(playgroundSessionCookie)
	if err == nil {
		_, err = s.PlaygroundSession.Authenticate(r.Context(), cookie.Value)
	}
	if err == nil {
		return true
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
	return false
}
