package http

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/javleds/api-engine/internal/api/http/dto"
	"github.com/javleds/api-engine/internal/execution/application/ports"
	"github.com/javleds/api-engine/internal/execution/domain"
)

func (s *Server) getV2Results(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r, "executions:read")
	if !ok {
		return
	}
	after, limit, err := v2PageQuery(r)
	if err != nil {
		presentError(w, err, "")
		return
	}
	through := int64(0)
	if values, exists := r.URL.Query()["through"]; exists {
		if len(values) != 1 {
			presentError(w, domain.ErrV2Cursor, "")
			return
		}
		through, err = strconv.ParseInt(values[0], 10, 64)
		if err != nil || through < 0 || strconv.FormatInt(through, 10) != values[0] {
			presentError(w, domain.ErrV2Cursor, "")
			return
		}
	}
	store, ok := s.V2Store.(ports.V2ResultStore)
	if !ok {
		writeError(w, 503, "STORAGE_UNAVAILABLE", "")
		return
	}
	page, err := store.ReadV2Results(r.Context(), principal.OwnerID, domain.ExecutionID(r.PathValue("id")), after, through, limit, s.Clock.Now())
	if err != nil {
		presentError(w, err, "")
		return
	}
	results := make([]dto.V2ResultResponse, 0, len(page.Results))
	base := s.executionLinks(r.PathValue("id")).Results
	for _, result := range page.Results {
		results = append(results, dto.V2ResultResponse{V2Result: result, Links: dto.V2ResultLinks{Result: base + "/" + url.PathEscape(result.NodeID)}})
	}
	response := dto.V2ResultPage{Results: results, NextCursor: page.NextCursor, LastSequence: page.LastSequence, HasMore: page.HasMore, Terminal: page.Terminal}
	for {
		encoded, err := json.Marshal(response)
		if err != nil {
			presentError(w, err, "")
			return
		}
		if len(encoded) <= 1<<20 {
			writeJSON(w, http.StatusOK, json.RawMessage(encoded))
			return
		}
		if len(response.Results) <= 1 {
			writeError(w, 503, "STORAGE_UNAVAILABLE", "")
			return
		}
		response.Results = response.Results[:len(response.Results)-1]
		response.HasMore = true
		response.NextCursor = response.Results[len(response.Results)-1].Sequence
	}
}

func (s *Server) getV2Result(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r, "executions:read")
	if !ok {
		return
	}
	store, ok := s.V2Store.(ports.V2ResultStore)
	if !ok {
		writeError(w, 503, "STORAGE_UNAVAILABLE", "")
		return
	}
	id, node := domain.ExecutionID(r.PathValue("id")), r.PathValue("nodeId")
	if _, err := store.ReadV2Result(r.Context(), principal.OwnerID, id, node, s.Clock.Now()); err != nil {
		presentError(w, err, "")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	controller := http.NewResponseController(w)
	defer controller.SetWriteDeadline(time.Time{})
	writer := bufio.NewWriterSize(v2ResultStreamWriter{ctx: r.Context(), response: w, controller: controller}, 32<<10)
	// Once streaming starts, errors deliberately leave out the final completion
	// marker. Appending an HTTP error would corrupt the result envelope.
	if err := store.WriteV2Result(r.Context(), principal.OwnerID, id, node, s.Clock.Now(), writer); err != nil {
		_ = writer.Flush()
		return
	}
	_ = writer.Flush()
}

type v2ResultStreamWriter struct {
	ctx        context.Context
	response   http.ResponseWriter
	controller *http.ResponseController
}

func (w v2ResultStreamWriter) Write(data []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	_ = w.controller.SetWriteDeadline(time.Now().Add(30 * time.Second))
	n, err := w.response.Write(data)
	if err != nil {
		return n, err
	}
	return n, w.controller.Flush()
}
