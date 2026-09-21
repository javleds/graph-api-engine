package http

import (
	"github.com/javleds/api-engine/internal/api/http/dto"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	"github.com/javleds/api-engine/internal/execution/domain"
	"net/http"
)

func (s *Server) apiVersion() int {
	return s.route.apiVersion
}
func (s *Server) matchesVersion(e domain.Execution) bool {
	version := e.APIVersion
	if version == 0 {
		version = 1
	}
	return version == s.apiVersion()
}
func (s *Server) validateVersionInput(w http.ResponseWriter, input dto.CreateExecutionRequest) bool {
	if s.apiVersion() == 2 && input.Webhook != nil && input.Webhook.Payload == "result" {
		presentError(w, &definition.ValidationError{Code: "INVALID_DEFINITION", Path: "/webhook/payload", Message: "V2 webhooks accept only status payloads."}, "")
		return false
	}
	return true
}
func (s *Server) registerV2(mux *http.ServeMux, protection *http.CrossOriginProtection) {
	base := s.route.executionBasePath
	create := http.Handler(http.HandlerFunc(s.createExecution))
	if protection != nil {
		create = protection.Handler(create)
	}
	mux.Handle("POST "+base, create)
	cancel := http.Handler(http.HandlerFunc(s.cancelExecution))
	if protection != nil {
		cancel = protection.Handler(cancel)
	}
	mux.Handle("POST "+base+"/{id}/cancel", cancel)
	mux.HandleFunc("GET "+base+"/{id}", s.getExecution)
	mux.HandleFunc("GET "+base+"/{id}/chunks", s.getV2Chunks)
	mux.HandleFunc("GET "+base+"/{id}/tree", s.getV2Tree)
	mux.HandleFunc("GET "+base+"/{id}/export", s.exportV2)
	mux.HandleFunc("GET "+base+"/{id}/results", s.getV2Results)
	mux.HandleFunc("GET "+base+"/{id}/results/{nodeId}", s.getV2Result)
	mux.HandleFunc("GET "+base+"/{id}/webhooks", s.getV2Webhooks)
}

func (s *Server) registerV1(mux *http.ServeMux) {
	base := s.route.executionBasePath
	mux.HandleFunc("POST "+base, s.createExecution)
	mux.HandleFunc("POST "+base+"/{id}/cancel", s.cancelExecution)
	mux.HandleFunc("GET "+base+"/{id}", s.getExecution)
	mux.HandleFunc("GET "+base+"/{id}/result", s.getResult)
}
