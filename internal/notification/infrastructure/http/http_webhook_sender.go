package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	fetchapp "github.com/javleds/api-engine/internal/fetch/application"
	fetchhttp "github.com/javleds/api-engine/internal/fetch/infrastructure/http"
	"github.com/javleds/api-engine/internal/notification/domain"
	"github.com/javleds/api-engine/internal/platform/clock"
	securityports "github.com/javleds/api-engine/internal/security/application/ports"
	"io"
	"net/http"
	"strconv"
	"time"
)

// HTTPWebhookSender owns a dedicated connection pool, without cache or flights.
type HTTPWebhookSender struct {
	Client  *http.Client
	Policy  securityports.EgressPolicy
	Clock   clock.Clock
	Options SenderOptions
}

func NewHTTPWebhookSender(policy securityports.EgressPolicy, options SenderOptions) *HTTPWebhookSender {
	if options.AttemptTimeout <= 0 {
		options.AttemptTimeout = 10 * time.Second
	}
	if options.ConnectTimeout <= 0 {
		options.ConnectTimeout = 5 * time.Second
	}
	if options.MaxResponseBytes <= 0 {
		options.MaxResponseBytes = 65536
	}
	if options.MaxConcurrent <= 0 {
		options.MaxConcurrent = 4
	}
	dialer := &fetchhttp.SafeDialer{Policy: policy, Timeout: options.ConnectTimeout}
	transport := &http.Transport{DialContext: dialer.DialContext, TLSHandshakeTimeout: options.ConnectTimeout, ResponseHeaderTimeout: options.AttemptTimeout, MaxIdleConns: options.MaxConcurrent, MaxIdleConnsPerHost: options.MaxConcurrent, IdleConnTimeout: 90 * time.Second, ForceAttemptHTTP2: true}
	return &HTTPWebhookSender{Client: &http.Client{Transport: transport, Timeout: options.AttemptTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, Policy: policy, Clock: clock.RealClock{}, Options: options}
}
func (s *HTTPWebhookSender) Close() { s.Client.CloseIdleConnections() }
func (s *HTTPWebhookSender) Send(ctx context.Context, delivery domain.WebhookDelivery) (domain.WebhookAttempt, error) {
	var target definition.Webhook
	if err := json.Unmarshal(delivery.Target, &target); err != nil {
		return domain.WebhookAttempt{ErrorCode: "WEBHOOK_INVALID_TARGET"}, nil
	}
	if s.Policy == nil || s.Policy.ValidateURL(target.URL) != nil {
		return domain.WebhookAttempt{ErrorCode: "EGRESS_DENIED"}, nil
	}
	if target.Method != "POST" && target.Method != "PUT" && target.Method != "PATCH" {
		return domain.WebhookAttempt{ErrorCode: "WEBHOOK_INVALID_METHOD"}, nil
	}
	request, err := http.NewRequestWithContext(ctx, target.Method, target.URL, bytes.NewReader(delivery.Payload))
	if err != nil {
		return domain.WebhookAttempt{ErrorCode: "WEBHOOK_INVALID_TARGET"}, nil
	}
	for key, value := range target.Headers {
		request.Header.Set(key, value)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "api-execution-engine/1.1")
	request.Header.Set("X-Engine-Event-Id", delivery.EventID)
	source := s.Clock
	if source == nil {
		source = clock.RealClock{}
	}
	timestamp := strconv.FormatInt(source.Now().Unix(), 10)
	request.Header.Set("X-Engine-Timestamp", timestamp)
	switch target.Auth.Type {
	case "bearer":
		request.Header.Set("Authorization", "Bearer "+target.Auth.Token)
	case "basic":
		request.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(target.Auth.Username+":"+target.Auth.Password)))
	case "header":
		request.Header.Set(target.Auth.Name, target.Auth.Value)
	case "hmac":
		request.Header.Set("X-Engine-Signature", Sign(target.Auth.Secret, delivery.EventID, timestamp, delivery.Payload))
	}
	response, err := s.Client.Do(request)
	if err != nil {
		return classifySendError(err), nil
	}
	defer response.Body.Close()
	// Drain only a bounded prefix; the webhook response is never retained or logged.
	maximum := s.Options.MaxResponseBytes
	if maximum <= 0 {
		maximum = 65536
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maximum))
	attempt := domain.WebhookAttempt{StatusCode: response.StatusCode}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return attempt, nil
	}
	attempt.ErrorCode = "WEBHOOK_HTTP_ERROR"
	switch response.StatusCode {
	case 408, 429, 500, 502, 503, 504:
		attempt.Retryable = true
	}
	if after, ok := fetchapp.RetryAfter(response.Header.Get("Retry-After"), source.Now()); ok {
		attempt.RetryAfter = source.Now().Add(after)
	}
	return attempt, nil
}
