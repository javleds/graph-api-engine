package http

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/javleds/api-engine/internal/api/http/dto"
	definitiondomain "github.com/javleds/api-engine/internal/definition/domain"
	executiondomain "github.com/javleds/api-engine/internal/execution/domain"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "internal error", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
func writeError(w http.ResponseWriter, status int, code, id string) {
	requestID := uuid.NewString()
	w.Header().Set("X-Request-Id", requestID)
	if code == "ENGINE_BUSY" {
		w.Header().Set("Retry-After", "2")
	}
	writeJSON(w, status, dto.ErrorResponse{Error: dto.PublicError{Code: code, Message: "The request could not be completed."}, RequestID: requestID, ID: id})
}
func presentError(w http.ResponseWriter, err error, id string) {
	var validation *definitiondomain.ValidationError
	if errors.As(err, &validation) {
		status := 422
		if validation.Code == "INVALID_JSON" {
			status = 400
		}
		writeError(w, status, validation.Code, id)
		return
	}
	var storage executiondomain.StorageError
	if errors.As(err, &storage) {
		status := 503
		switch string(storage) {
		case "EXECUTION_NOT_FOUND":
			status = 404
		case "IDEMPOTENCY_CONFLICT", "RESULT_NOT_READY", "EXECUTION_FAILED":
			status = 409
		case "RESULT_EXPIRED":
			status = 410
		}
		writeError(w, status, string(storage), id)
		return
	}
	writeError(w, 503, "STORAGE_UNAVAILABLE", id)
}
