package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestV2ResultStreamHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	response := httptest.NewRecorder()
	writer := v2ResultStreamWriter{ctx: ctx, response: response, controller: http.NewResponseController(response)}
	n, err := writer.Write([]byte(`{"complete":true}`))
	if n != 0 || !errors.Is(err, context.Canceled) || response.Body.Len() != 0 {
		t.Fatalf("canceled response wrote %d bytes: %v", n, err)
	}
}

func TestV2ResultStreamFlushesChunks(t *testing.T) {
	response := httptest.NewRecorder()
	writer := v2ResultStreamWriter{ctx: context.Background(), response: response, controller: http.NewResponseController(response)}
	data := []byte(`{"data":`)
	n, err := writer.Write(data)
	if n != len(data) || err != nil || !response.Flushed {
		t.Fatalf("chunk not flushed: n=%d err=%v flushed=%v", n, err, response.Flushed)
	}
}
