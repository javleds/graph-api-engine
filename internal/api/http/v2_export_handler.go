package http

import (
	"encoding/json"
	"github.com/javleds/api-engine/internal/execution/domain"
	"net/http"
	"strconv"
	"time"
)

// exportV2 streams a fixed prefix. The trailer distinguishes a complete snapshot
// from a disconnected or interrupted download, even for a still-running job.
func (s *Server) exportV2(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r, "executions:read")
	if !ok {
		return
	}
	id := domain.ExecutionID(r.PathValue("id"))
	// Reading the first page checks ownership, mode and retention before headers.
	page, err := s.V2Store.ReadV2Chunks(r.Context(), principal.OwnerID, id, 0, 100, 1<<20, s.Clock.Now())
	if err != nil {
		presentError(w, err, "")
		return
	}
	cutoff, err := strconv.ParseInt(page.LastSequence, 10, 64)
	if err != nil {
		presentError(w, err, "")
		return
	}
	terminal := page.Terminal
	controller := http.NewResponseController(w)
	defer controller.SetWriteDeadline(time.Time{})
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", `attachment; filename="execution.ndjson"`)
	encoder := json.NewEncoder(w)
	write := func(value any) bool {
		_ = controller.SetWriteDeadline(time.Now().Add(30 * time.Second))
		if err := encoder.Encode(value); err != nil {
			return false
		}
		return controller.Flush() == nil
	}
	if !write(map[string]any{"type": "export", "version": 2, "executionId": id, "lastSequence": page.LastSequence, "terminal": terminal}) {
		return
	}
	after := int64(0)
	for after < cutoff {
		for _, chunk := range page.Chunks {
			var envelope domain.V2Chunk
			if json.Unmarshal(chunk, &envelope) != nil {
				return
			}
			sequence, err := strconv.ParseInt(envelope.Sequence, 10, 64)
			if err != nil || sequence <= after {
				return
			}
			if sequence > cutoff {
				break
			}
			if !write(chunk) {
				return
			}
			after = sequence
		}
		if after >= cutoff {
			break
		}
		if len(page.Chunks) == 0 {
			return
		}
		page, err = s.V2Store.ReadV2Chunks(r.Context(), principal.OwnerID, id, after, 100, 1<<20, s.Clock.Now())
		if err != nil {
			return
		}
	}
	write(map[string]any{"type": "export_end", "version": 2, "executionId": id, "lastSequence": strconv.FormatInt(cutoff, 10), "terminal": terminal, "complete": true})
}
