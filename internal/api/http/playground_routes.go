package http

import (
	"github.com/javleds/api-engine/internal/api/http/playground"
	"net/http"
)

func (s *Server) registerPlayground(mux *http.ServeMux) {
	playground.Register(mux)
	s.registerPlaygroundSession(mux)
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
