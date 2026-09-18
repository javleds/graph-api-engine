package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	"github.com/javleds/api-engine/internal/notification/domain"
	"github.com/javleds/api-engine/internal/security/infrastructure/egress"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func localSender(t *testing.T, server *httptest.Server) *HTTPWebhookSender {
	t.Helper()
	policy, err := egress.NewPolicy([]string{server.URL}, []string{"127.0.0.0/8"}, true)
	if err != nil {
		t.Fatal(err)
	}
	sender := NewHTTPWebhookSender(policy, SenderOptions{})
	t.Cleanup(sender.Close)
	return sender
}
func delivery(target definition.Webhook) domain.WebhookDelivery {
	raw, _ := json.Marshal(target)
	return domain.WebhookDelivery{EventID: "event-id", Target: raw, Payload: []byte("{\"id\":1, \"detail\":null}\n")}
}
func TestHMACSignsExactBodyAndRenewsTimestamp(t *testing.T) {
	captured := make(chan capturedRequest, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured <- capturedRequest{headers: r.Header.Clone(), body: string(body), method: r.Method}
		w.WriteHeader(204)
	}))
	defer server.Close()
	sender := localSender(t, server)
	d := delivery(definition.Webhook{URL: server.URL, Method: "POST", Auth: definition.Authentication{Type: "hmac", Secret: "shared-secret"}})
	for _, seconds := range []int64{100, 200} {
		sender.Clock = fixedClock{time.Unix(seconds, 0)}
		attempt, err := sender.Send(context.Background(), d)
		if err != nil || !attempt.Accepted() {
			t.Fatal(attempt, err)
		}
	}
	first, second := <-captured, <-captured
	if first.body != string(d.Payload) || first.headers.Get("X-Engine-Event-Id") != d.EventID || first.headers.Get("X-Engine-Timestamp") == second.headers.Get("X-Engine-Timestamp") || first.headers.Get("X-Engine-Signature") == second.headers.Get("X-Engine-Signature") {
		t.Fatal(first, second)
	}
	mac := hmac.New(sha256.New, []byte("shared-secret"))
	_, _ = mac.Write([]byte(d.EventID + ".100."))
	_, _ = mac.Write(d.Payload)
	if first.headers.Get("X-Engine-Signature") != "v1="+hex.EncodeToString(mac.Sum(nil)) {
		t.Fatal("body was not signed exactly")
	}
}
func TestRedirectNotFollowed(t *testing.T) {
	var redirected atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/next" {
			redirected.Add(1)
		}
		http.Redirect(w, r, "/next", 302)
	}))
	defer server.Close()
	sender := localSender(t, server)
	attempt, err := sender.Send(context.Background(), delivery(definition.Webhook{URL: server.URL, Method: "POST"}))
	if err != nil || attempt.StatusCode != 302 || attempt.Retryable || redirected.Load() != 0 {
		t.Fatal(attempt, err)
	}
}
func TestHTTPRetryClassesAndRetryAfter(t *testing.T) {
	status := atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(int(status.Load()))
	}))
	defer server.Close()
	sender := localSender(t, server)
	now := time.Unix(100, 0)
	sender.Clock = fixedClock{now}
	d := delivery(definition.Webhook{URL: server.URL, Method: "PATCH"})
	for _, code := range []int{400, 401, 403, 404, 422, 408, 429, 500, 502, 503, 504} {
		status.Store(int32(code))
		attempt, err := sender.Send(context.Background(), d)
		if err != nil {
			t.Fatal(err)
		}
		expected := code == 408 || code == 429 || code == 500 || code == 502 || code == 503 || code == 504
		if attempt.Retryable != expected || !attempt.RetryAfter.Equal(now.Add(time.Minute)) {
			t.Fatal(code, attempt)
		}
	}
}
func TestBearerBasicAndCustomAuthentication(t *testing.T) {
	headers := make(chan http.Header, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { headers <- r.Header.Clone(); w.WriteHeader(200) }))
	defer server.Close()
	sender := localSender(t, server)
	authentications := []definition.Authentication{{Type: "bearer", Token: "token"}, {Type: "basic", Username: "user", Password: "pass"}, {Type: "header", Name: "X-API-Key", Value: "key"}}
	for _, auth := range authentications {
		attempt, err := sender.Send(context.Background(), delivery(definition.Webhook{URL: server.URL, Method: "PUT", Auth: auth}))
		if err != nil || !attempt.Accepted() {
			t.Fatal(attempt, err)
		}
	}
	if (<-headers).Get("Authorization") != "Bearer token" || (<-headers).Get("Authorization") != "Basic dXNlcjpwYXNz" || (<-headers).Get("X-API-Key") != "key" {
		t.Fatal("wrong authentication")
	}
}
func TestBlockedDestinationIsNotSent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("blocked destination reached") }))
	defer server.Close()
	policy, err := egress.NewPolicy([]string{server.URL}, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	sender := NewHTTPWebhookSender(policy, SenderOptions{})
	defer sender.Close()
	attempt, err := sender.Send(context.Background(), delivery(definition.Webhook{URL: server.URL, Method: "POST"}))
	if err != nil || attempt.Accepted() || attempt.Retryable {
		t.Fatal(attempt, err)
	}
}
func TestLargeResponseDoesNotInvalidateAcceptance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, strings.Repeat("x", 10000)) }))
	defer server.Close()
	sender := localSender(t, server)
	sender.Options.MaxResponseBytes = 32
	attempt, err := sender.Send(context.Background(), delivery(definition.Webhook{URL: server.URL, Method: "POST"}))
	if err != nil || !attempt.Accepted() {
		t.Fatal(attempt, err)
	}
}
func TestTLSFailureIsPermanent(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("untrusted TLS reached handler") }))
	defer server.Close()
	sender := localSender(t, server)
	attempt, err := sender.Send(context.Background(), delivery(definition.Webhook{URL: server.URL, Method: "POST"}))
	if err != nil || attempt.Retryable || attempt.ErrorCode != "WEBHOOK_TLS_ERROR" {
		t.Fatal(attempt, err)
	}
}
