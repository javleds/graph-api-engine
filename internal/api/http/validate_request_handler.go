package http

import (
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/javleds/api-engine/internal/api/http/dto"
	"github.com/javleds/api-engine/internal/api/http/mapper"
	"github.com/javleds/api-engine/internal/definition/infrastructure/schema"
)

// validateRequest only compiles the input; it never reserves capacity or creates a job.
func (s *Server) validateRequest(w http.ResponseWriter, r *http.Request) {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "")
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
	if _, err := mapper.Compile(input, s.Limits); err != nil {
		presentError(w, err, "")
		return
	}
	writeJSON(w, http.StatusOK, dto.RequestValidationResponse{Valid: true})
}
