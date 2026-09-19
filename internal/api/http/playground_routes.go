package http

import (
	"github.com/javleds/api-engine/internal/api/http/playground"
	"net/http"
)

func (s *Server) registerPlayground(mux *http.ServeMux) {
	playground.Register(mux)
	if s.PlaygroundAuth == nil {
		return
	}
	local := *s
	local.Auth = s.PlaygroundAuth
	local.ExecutionBasePath = "/playground/api/executions"
	protection := http.NewCrossOriginProtection()
	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "")
	}))
	mux.Handle("POST /playground/api/executions", protection.Handler(http.HandlerFunc(local.createExecution)))
	mux.HandleFunc("GET /playground/api/executions/{id}", local.getExecution)
	mux.HandleFunc("GET /playground/api/executions/{id}/result", local.getResult)
}
