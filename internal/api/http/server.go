package http

import (
	"context"
	definitionapp "github.com/javleds/api-engine/internal/definition/application"
	definitiondomain "github.com/javleds/api-engine/internal/definition/domain"
	"github.com/javleds/api-engine/internal/execution/application"
	"github.com/javleds/api-engine/internal/execution/application/ports"
	notificationports "github.com/javleds/api-engine/internal/notification/application/ports"
	"github.com/javleds/api-engine/internal/platform/clock"
	securityports "github.com/javleds/api-engine/internal/security/application/ports"
	"net/http"
)

type Server struct {
	PlaygroundAuth       securityports.PrincipalAuthenticator
	ExecutionBasePath    string
	ValidateQuotas       func(context.Context, definitiondomain.CompiledDefinition) error
	MetricsPath          string
	WebhookStatus        notificationports.WebhookStatusReader
	Create               *application.CreateExecution
	Reader               ports.ExecutionReader
	Results              ports.ResultReader
	Auth                 securityports.PrincipalAuthenticator
	Clock                clock.Clock
	Limits               definitionapp.ValidationLimits
	MaxRequestBytes      int64
	MaxJSONDepth         int
	ValidateDestinations func(definitiondomain.CompiledDefinition) error
	Ready                func(context.Context) bool
	Metrics              http.Handler
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.registerPlayground(mux)
	mux.HandleFunc("POST /v1/executions", s.createExecution)
	mux.HandleFunc("GET /v1/executions/{id}", s.getExecution)
	mux.HandleFunc("GET /v1/executions/{id}/result", s.getResult)
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "live"}) })
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		if s.Ready != nil && !s.Ready(r.Context()) {
			writeJSON(w, 503, map[string]string{"status": "not_ready"})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ready"})
	})
	if s.Metrics != nil {
		path := s.MetricsPath
		if path == "" {
			path = "/metrics"
		}
		mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
			if _, ok := s.authorize(w, r, "metrics:read"); ok {
				s.Metrics.ServeHTTP(w, r)
			}
		})
	}
	return mux
}
