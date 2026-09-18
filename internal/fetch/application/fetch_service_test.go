package application

import (
	"context"
	"errors"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"testing"
	"time"
)

func TestFetchRetriesOnlyTransientFailures(t *testing.T) {
	for _, status := range []int{200, 204, 301, 400, 401, 403, 404, 408, 422, 429, 500, 502, 503, 504} {
		t.Run(string(rune(status)), func(t *testing.T) {
			attempts := 0
			service := NewFetchService(transportStub(func(context.Context, domain.OutboundRequest) (domain.RawResponse, error) {
				attempts++
				return domain.RawResponse{StatusCode: status, Body: []byte(`{}`)}, nil
			}))
			_, err := service.Fetch(context.Background(), domain.FetchRequest{Policy: domain.FetchPolicy{MaxAttempts: 2, BaseDelay: time.Nanosecond, MaxBackoff: time.Nanosecond}})
			want := 1
			if status == 408 || status == 429 || status == 500 || status == 502 || status == 503 || status == 504 {
				want = 2
			}
			if attempts != want {
				t.Fatalf("status %d attempts %d want %d", status, attempts, want)
			}
			if status >= 300 && err == nil {
				t.Fatal("expected typed failure")
			}
		})
	}
}

func TestFetchRecoveredRetryDoesNotFail(t *testing.T) {
	attempts := 0
	service := NewFetchService(transportStub(func(context.Context, domain.OutboundRequest) (domain.RawResponse, error) {
		attempts++
		if attempts == 1 {
			return domain.RawResponse{StatusCode: 503}, nil
		}
		return domain.RawResponse{StatusCode: 200, Body: []byte(`[]`)}, nil
	}))
	result, err := service.Fetch(context.Background(), domain.FetchRequest{Policy: domain.FetchPolicy{BaseDelay: time.Nanosecond, MaxBackoff: time.Nanosecond}})
	if err != nil || result.Response.Attempts != 2 {
		t.Fatalf("result %+v error %v", result, err)
	}
}

func TestFetchRetryAfterExceedingDeadline(t *testing.T) {
	service := NewFetchService(transportStub(func(context.Context, domain.OutboundRequest) (domain.RawResponse, error) {
		return domain.RawResponse{StatusCode: 429, Headers: map[string][]string{"Retry-After": {"17"}}}, nil
	}))
	_, err := service.Fetch(context.Background(), domain.FetchRequest{Policy: domain.FetchPolicy{OperationTimeout: time.Second}})
	var failure *domain.UpstreamFailure
	if !errors.As(err, &failure) || failure.Code != "THROTTLE_WAIT_EXCEEDS_DEADLINE" || failure.Attempts != 1 || failure.RetryExhausted {
		t.Fatalf("%+v", err)
	}
}

func TestFetchPanicIsSupervised(t *testing.T) {
	service := NewFetchService(transportStub(func(context.Context, domain.OutboundRequest) (domain.RawResponse, error) {
		panic("sensitive panic value")
	}))
	_, err := service.Fetch(context.Background(), domain.FetchRequest{})
	if err == nil || err.Error() != "INTERNAL_WORKER_PANIC" {
		t.Fatalf("unexpected failure %v", err)
	}
}

func TestRequestURLPreservesPrefixAndEscaping(t *testing.T) {
	got, err := BuildRequestURL("https://example.com/api/v1", "/items/a%2Fb", map[string]string{"q": "a&b"})
	if err != nil || got != "https://example.com/api/v1/items/a%2Fb?q=a%26b" {
		t.Fatalf("%s %v", got, err)
	}
}

func TestRetryAfter(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, raw := range []string{"17", now.Add(17 * time.Second).Format("Mon, 02 Jan 2006 15:04:05 GMT")} {
		delay, ok := RetryAfter(raw, now)
		if !ok || delay != 17*time.Second {
			t.Fatalf("%s %v %v", raw, delay, ok)
		}
	}
}

func TestFetchRejectsJSONDepthAndValueBudgets(t *testing.T) {
	service := NewFetchService(transportStub(func(context.Context, domain.OutboundRequest) (domain.RawResponse, error) {
		return domain.RawResponse{StatusCode: 200, Body: []byte(`[[[1]]]`)}, nil
	}))
	service.MaxJSONDepth = 2
	if _, err := service.Fetch(context.Background(), domain.FetchRequest{}); err == nil || err.Error() != "UPSTREAM_INVALID_RESPONSE" {
		t.Fatalf("depth: %v", err)
	}
	service.MaxJSONDepth = 4
	service.MaxJSONValues = 2
	if _, err := service.Fetch(context.Background(), domain.FetchRequest{}); err == nil || err.Error() != "UPSTREAM_INVALID_RESPONSE" {
		t.Fatalf("values: %v", err)
	}
}

func TestLogicalTimeoutPreservesPhysicalAttempts(t *testing.T) {
	service := NewFetchService(transportStub(func(ctx context.Context, _ domain.OutboundRequest) (domain.RawResponse, error) {
		<-ctx.Done()
		return domain.RawResponse{}, ctx.Err()
	}))
	_, err := service.Fetch(context.Background(), domain.FetchRequest{Policy: domain.FetchPolicy{OperationTimeout: time.Millisecond, AttemptTimeout: time.Second}})
	var failure *domain.UpstreamFailure
	if !errors.As(err, &failure) || failure.Code != "UPSTREAM_TIMEOUT" || failure.Attempts != 1 || failure.RetryExhausted {
		t.Fatalf("%+v", err)
	}
}

func TestRequestURLRejectsTraversalAfterBinding(t *testing.T) {
	for _, path := range []string{"/items/..", "/items/%2e%2e", "/items/a%2F..%2Fsecret", "/items/%5csecret"} {
		if _, err := BuildRequestURL("https://example.com/api", path, nil); err == nil {
			t.Fatalf("accepted %s", path)
		}
	}
}
