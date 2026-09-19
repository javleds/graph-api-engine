package http

import (
	"context"
	"errors"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"github.com/javleds/api-engine/internal/security/infrastructure/egress"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTransportBoundsBodyAndDoesNotRedirect(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/other", 302)
			return
		}
		_, _ = w.Write([]byte(`{"oversized":true}`))
	}))
	defer server.Close()
	policy, err := egress.NewPolicy([]string{server.URL}, []string{"127.0.0.0/8"}, true)
	if err != nil {
		t.Fatal(err)
	}
	transport, err := NewNetHTTPTransport(policy, TransportOptions{MaxResponseBytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	defer transport.CloseIdleConnections()
	_, err = transport.Execute(context.Background(), domain.OutboundRequest{Method: "GET", URL: server.URL})
	var failure *domain.UpstreamFailure
	if !errors.As(err, &failure) || failure.Code != "UPSTREAM_RESPONSE_TOO_LARGE" {
		t.Fatalf("%v", err)
	}
	transport.maxResponseBytes = 1024
	response, err := transport.Execute(context.Background(), domain.OutboundRequest{Method: "GET", URL: server.URL + "/redirect"})
	if err != nil || response.StatusCode != 302 || hits != 2 {
		t.Fatalf("status %d hits %d error %v", response.StatusCode, hits, err)
	}
}

func TestResolvedPrivateAddressReturnsEgressDenied(t *testing.T) {
	policy, err := egress.NewPolicy([]string{"https://127.0.0.1"}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	transport, err := NewNetHTTPTransport(policy, TransportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer transport.CloseIdleConnections()
	_, err = transport.Execute(context.Background(), domain.OutboundRequest{Method: "GET", URL: "https://127.0.0.1"})
	var failure *domain.UpstreamFailure
	if !errors.As(err, &failure) || failure.Code != "EGRESS_DENIED" {
		t.Fatalf("%v", err)
	}
}
