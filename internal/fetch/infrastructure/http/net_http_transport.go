package http

import (
	"bytes"
	"context"
	"errors"
	"github.com/javleds/api-engine/internal/fetch/domain"
	securityports "github.com/javleds/api-engine/internal/security/application/ports"
	"io"
	"net/http"
	"time"
)

type NetHTTPTransport struct {
	client           *http.Client
	policy           securityports.EgressPolicy
	maxResponseBytes int64
}

func NewNetHTTPTransport(policy securityports.EgressPolicy, options TransportOptions) (*NetHTTPTransport, error) {
	if policy == nil {
		return nil, errors.New("egress policy required")
	}
	if options.MaxResponseBytes <= 0 {
		options.MaxResponseBytes = 4 << 20
	}
	if options.ConnectTimeout <= 0 {
		options.ConnectTimeout = 5 * time.Second
	}
	if options.TLSHandshakeTimeout <= 0 {
		options.TLSHandshakeTimeout = 5 * time.Second
	}
	if options.ResponseHeaderTimeout <= 0 {
		options.ResponseHeaderTimeout = 15 * time.Second
	}
	if options.MaxIdleConnections <= 0 {
		options.MaxIdleConnections = 100
	}
	if options.MaxIdleConnectionsPerHost <= 0 {
		options.MaxIdleConnectionsPerHost = 20
	}
	if options.IdleConnectionTimeout <= 0 {
		options.IdleConnectionTimeout = 90 * time.Second
	}
	dialer := &SafeDialer{Policy: policy, Timeout: options.ConnectTimeout}
	transport := &http.Transport{DialContext: dialer.DialContext, TLSHandshakeTimeout: options.TLSHandshakeTimeout, ResponseHeaderTimeout: options.ResponseHeaderTimeout, MaxIdleConns: options.MaxIdleConnections, MaxIdleConnsPerHost: options.MaxIdleConnectionsPerHost, IdleConnTimeout: options.IdleConnectionTimeout, ForceAttemptHTTP2: true}
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &NetHTTPTransport{client: client, policy: policy, maxResponseBytes: options.MaxResponseBytes}, nil
}

func (t *NetHTTPTransport) Execute(ctx context.Context, input domain.OutboundRequest) (domain.RawResponse, error) {
	if err := t.policy.ValidateURL(input.URL); err != nil {
		return domain.RawResponse{}, &domain.UpstreamFailure{Code: "EGRESS_DENIED"}
	}
	req, err := http.NewRequestWithContext(ctx, input.Method, input.URL, bytes.NewReader(input.Body))
	if err != nil {
		return domain.RawResponse{}, &domain.UpstreamFailure{Code: "UPSTREAM_INVALID_RESPONSE"}
	}
	req.Header = http.Header(input.Headers).Clone()
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	req.Header.Set("User-Agent", "api-execution-engine/1.1")
	response, err := t.client.Do(req)
	if err != nil {
		return domain.RawResponse{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, t.maxResponseBytes+1))
	if err != nil {
		return domain.RawResponse{}, err
	}
	if int64(len(body)) > t.maxResponseBytes {
		return domain.RawResponse{}, &domain.UpstreamFailure{Code: "UPSTREAM_RESPONSE_TOO_LARGE", HTTPStatus: response.StatusCode}
	}
	return domain.RawResponse{StatusCode: response.StatusCode, Headers: map[string][]string(response.Header.Clone()), Body: body, FetchedAt: time.Now(), Attempts: 1}, nil
}

func (t *NetHTTPTransport) CloseIdleConnections() { t.client.CloseIdleConnections() }
