package http

import (
	"encoding/json"
	"errors"
	"github.com/javleds/api-engine/internal/api/http/dto"
	"github.com/javleds/api-engine/internal/api/http/mapper"
	"github.com/javleds/api-engine/internal/definition/infrastructure/schema"
	"github.com/javleds/api-engine/internal/execution/application"
	"io"
	"mime"
	"net/http"
	"regexp"
)

func (s *Server) createExecution(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r, "executions:create")
	if !ok {
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		writeError(w, 415, "UNSUPPORTED_MEDIA_TYPE", "")
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if _, supplied := r.Header["Idempotency-Key"]; supplied && !regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`).MatchString(key) {
		writeError(w, 422, "INVALID_DEFINITION", "")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, s.MaxRequestBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, 413, "REQUEST_TOO_LARGE", "")
		} else {
			writeError(w, 400, "INVALID_JSON", "")
		}
		return
	}
	input, err := schema.ParseVersion(body, s.MaxJSONDepth, s.apiVersion())
	if err != nil {
		presentError(w, err, "")
		return
	}
	if !s.validateVersionInput(w, input) {
		return
	}
	compiled, err := mapper.Compile(input, s.Limits)
	if err != nil {
		presentError(w, err, "")
		return
	}
	if s.ValidateDestinations != nil {
		if err := s.ValidateDestinations(*compiled); err != nil {
			writeError(w, 422, "EGRESS_DENIED", "")
			return
		}
	}
	if s.ValidateQuotas != nil {
		if err := s.ValidateQuotas(r.Context(), *compiled); err != nil {
			presentError(w, err, "")
			return
		}
	}
	snapshot, err := json.Marshal(compiled)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "")
		return
	}
	identityDefinition, err := mapper.FingerprintDefinition(*compiled, s.apiVersion())
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "")
		return
	}
	result, err := s.Create.Execute(r.Context(), application.CreateExecutionCommand{APIVersion: s.apiVersion(), OwnerID: principal.OwnerID, IdempotencyKey: key, Definition: snapshot, FingerprintDefinition: identityDefinition, WebhookEnabled: compiled.Webhook != nil})
	if err != nil {
		presentError(w, err, string(result.Execution.ID))
		return
	}
	execution := result.Execution
	status := 202
	if result.Replay && execution.Status != "pending" && execution.Status != "running" {
		status = 200
	}
	links := s.executionLinks(string(execution.ID))
	w.Header().Set("Location", links.Self)
	writeJSON(w, status, dto.CreateExecutionResponse{ID: execution.ID, Status: execution.Status, Links: links})
}
