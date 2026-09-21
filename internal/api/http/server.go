package http

import (
	"context"
	definitionapp "github.com/javleds/api-engine/internal/definition/application"
	definitiondomain "github.com/javleds/api-engine/internal/definition/domain"
	"github.com/javleds/api-engine/internal/execution/application"
	"github.com/javleds/api-engine/internal/execution/application/ports"
	notificationports "github.com/javleds/api-engine/internal/notification/application/ports"
	"github.com/javleds/api-engine/internal/platform/clock"
	platformconfig "github.com/javleds/api-engine/internal/platform/config"
	securityports "github.com/javleds/api-engine/internal/security/application/ports"
	"net/http"
)

type Server struct {
	Configuration          *platformconfig.Config
	Cancel                 *application.CancelExecution
	V2Store                ports.V2Store
	PlaygroundAuth         securityports.PrincipalAuthenticator
	PlaygroundSession      securityports.PlaygroundSession
	PlaygroundCookieName   string
	PlaygroundCookieSecure bool
	ValidateQuotas         func(context.Context, definitiondomain.CompiledDefinition) error
	MetricsPath            string
	WebhookStatus          notificationports.WebhookStatusReader
	Create                 *application.CreateExecution
	Reader                 ports.ExecutionReader
	Results                ports.ResultReader
	Auth                   securityports.PrincipalAuthenticator
	Clock                  clock.Clock
	Limits                 definitionapp.ValidationLimits
	MaxRequestBytes        int64
	MaxJSONDepth           int
	ValidateDestinations   func(definitiondomain.CompiledDefinition) error
	Ready                  func(context.Context) bool
	Metrics                http.Handler
	ClearCache             func(context.Context) (int64, error)
	route                  routeDefinition
}

func (s *Server) scopedRoute(definition routeDefinition, auth securityports.PrincipalAuthenticator, cookieName string) *Server {
	scoped := *s
	scoped.route = definition
	scoped.Auth = auth
	scoped.PlaygroundCookieName = cookieName
	return &scoped
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.registerPlayground(mux)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if !s.uiEnabled() {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/playground", http.StatusSeeOther)
	})
	v1 := s.scopedRoute(routeDefinition{apiVersion: 1, executionBasePath: "/v1/executions"}, s.Auth, "")
	v1.registerV1(mux)
	v2 := s.scopedRoute(routeDefinition{apiVersion: 2, executionBasePath: "/v2/executions"}, s.Auth, "")
	v2.registerV2(mux, nil)
	mux.HandleFunc("POST /v1/cache/clear", v1.clearCache)
	mux.HandleFunc("POST /v2/cache/clear", v2.clearCache)
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
