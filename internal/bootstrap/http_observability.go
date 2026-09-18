package bootstrap

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/javleds/api-engine/internal/api/http/dto"
	"net/http"
	"time"
)

func (a *Application) observeHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.handlerMu.Lock()
		if a.handlersClosed {
			a.handlerMu.Unlock()
			http.Error(w, "service shutting down", http.StatusServiceUnavailable)
			return
		}
		a.handlers.Add(1)
		a.handlerMu.Unlock()
		defer a.handlers.Done()
		start := time.Now()
		observed := &httpResponseObserver{ResponseWriter: w}
		w.Header().Set("X-Request-Id", uuid.NewString())
		defer func() {
			if recover() != nil {
				a.Logger.Error("HTTP handler panic")
				if observed.status == 0 {
					observed.Header().Set("Content-Type", "application/json")
					observed.Header().Set("Cache-Control", "no-store")
					observed.WriteHeader(500)
					_ = json.NewEncoder(observed).Encode(dto.ErrorResponse{Error: dto.PublicError{Code: "INTERNAL_ERROR", Message: "The request could not be completed."}, RequestID: w.Header().Get("X-Request-Id")})
				}
			}
		}()
		next.ServeHTTP(observed, r)
		if r.Method == "POST" && r.URL.Path == "/v1/executions" {
			if observed.status == 202 || observed.status == 200 {
				a.Metrics.Add("creation_requests_accepted_total", 1)
			} else {
				a.Metrics.Add("executions_rejected_total", 1)
			}
		}
		a.Logger.Info("HTTP request", "request_id", w.Header().Get("X-Request-Id"), "method", r.Method, "status", observed.status, "duration_ms", time.Since(start).Milliseconds())
	})
}
