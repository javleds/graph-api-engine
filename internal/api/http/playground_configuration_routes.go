package http

import (
	"encoding/json"
	"net/http"

	platformconfig "github.com/javleds/api-engine/internal/platform/config"
)

type playgroundConfigurationResponse struct {
	Current  map[string]any `json:"current"`
	Defaults map[string]any `json:"defaults"`
}

// registerPlaygroundConfiguration exposes the effective non-secret service
// configuration to the authenticated operator console. Secrets and references
// to secret files are omitted before the response is written.
func (s *Server) registerPlaygroundConfiguration(mux *http.ServeMux) {
	if s.PlaygroundSession == nil || s.Configuration == nil {
		return
	}
	mux.HandleFunc("GET /playground/api/configuration", s.playgroundConfiguration)
}

func (s *Server) playgroundConfiguration(w http.ResponseWriter, r *http.Request) {
	local := s.scopedRoute(routeDefinition{apiVersion: 1, executionBasePath: "/playground/api/executions"}, s.PlaygroundSession, playgroundSessionCookie)
	if _, ok := local.authorize(w, r, "executions:read"); !ok {
		return
	}
	writeJSON(w, http.StatusOK, playgroundConfigurationResponse{
		Current:  publicConfiguration(*s.Configuration),
		Defaults: publicConfiguration(platformconfig.Default()),
	})
}

func publicConfiguration(configuration platformconfig.Config) map[string]any {
	encoded, err := json.Marshal(configuration)
	if err != nil {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal(encoded, &result); err != nil {
		return map[string]any{}
	}
	security, _ := result["security"].(map[string]any)
	delete(security, "principalsFile")
	delete(security, "identityKeyFile")
	delete(security, "encryption")
	delete(security, "playground")
	return result
}
