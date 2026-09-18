package local

import (
	"context"
	"errors"
	"github.com/javleds/api-engine/internal/fetch/application"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"sync/atomic"
	"testing"
	"time"
)

func TestSharedFlightTransfersAttemptBudgetAndRetryAccounting(t *testing.T) {
	coordinator := NewLocalFlightCoordinator(2, time.Minute)
	entered := make(chan struct{})
	release := make(chan struct{})
	var physical atomic.Int32
	raw := application.NewFetchService(pipelineTransport(func(context.Context, domain.OutboundRequest) (domain.RawResponse, error) {
		if physical.Add(1) == 1 {
			close(entered)
			<-release
			return domain.RawResponse{StatusCode: 503}, nil
		}
		return domain.RawResponse{StatusCode: 200, Body: []byte(`[]`)}, nil
	}))
	service := application.NewCachingFetchService(raw, nil, []byte("stable-key"))
	service.Flights = coordinator
	first := domain.FetchRequest{Request: domain.OutboundRequest{Method: "GET", URL: "https://example.com/items"}, Policy: domain.FetchPolicy{MaxAttempts: 2, BaseDelay: time.Nanosecond, MaxBackoff: time.Nanosecond}}
	second := first
	var firstSent, secondSent, secondRetries atomic.Int32
	first.BeforeAttempt = func(context.Context) error {
		if firstSent.Load() == 1 {
			return domain.AttemptBudgetExceeded{}
		}
		return nil
	}
	first.OnAttempt = func() { firstSent.Add(1) }
	second.BeforeAttempt = func(context.Context) error {
		if secondSent.Load() >= 2 {
			return domain.AttemptBudgetExceeded{}
		}
		return nil
	}
	second.OnAttempt = func() { secondSent.Add(1) }
	second.OnRetry = func() { secondRetries.Add(1) }
	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)
	go func() { _, err := service.Fetch(context.Background(), first); firstDone <- err }()
	<-entered
	go func() {
		result, err := service.Fetch(context.Background(), second)
		if err == nil && result.Response.Attempts != 2 {
			t.Error("shared outcome lost physical attempt count")
		}
		secondDone <- err
	}()
	key, err := application.RequestKey([]byte("stable-key"), first)
	if err != nil {
		t.Fatal(err)
	}
	awaitSubscribers(t, coordinator, application.FlightKey([]byte("stable-key"), key, first), 2)
	close(release)
	if err = <-firstDone; !errors.Is(err, domain.AttemptBudgetExceeded{}) {
		t.Fatalf("exhausted subscriber: %v", err)
	}
	if err = <-secondDone; err != nil {
		t.Fatalf("remaining subscriber: %v", err)
	}
	if physical.Load() != 2 || firstSent.Load() != 1 || secondSent.Load() != 1 || secondRetries.Load() != 1 {
		t.Fatalf("physical=%d first=%d second=%d retries=%d", physical.Load(), firstSent.Load(), secondSent.Load(), secondRetries.Load())
	}
}

func TestTwentySubscribersConsumeOnePhysicalAdmission(t *testing.T) {
	coordinator := NewLocalFlightCoordinator(1, time.Minute)
	entered := make(chan struct{})
	release := make(chan struct{})
	var physical atomic.Int32
	raw := application.NewFetchService(pipelineTransport(func(context.Context, domain.OutboundRequest) (domain.RawResponse, error) {
		physical.Add(1)
		close(entered)
		<-release
		return domain.RawResponse{StatusCode: 200, Body: []byte(`[]`)}, nil
	}))
	gate := &countedGate{gate: readyGate(1)}
	raw.Gate = gate
	service := application.NewCachingFetchService(raw, nil, []byte("key"))
	service.Flights = coordinator
	input := domain.FetchRequest{Request: domain.OutboundRequest{Method: "GET", URL: "https://example.com/items"}}
	done := make(chan error, 20)
	for i := 0; i < 20; i++ {
		go func() { _, err := service.Fetch(context.Background(), input); done <- err }()
	}
	<-entered
	key, err := application.RequestKey([]byte("key"), input)
	if err != nil {
		t.Fatal(err)
	}
	awaitSubscribers(t, coordinator, application.FlightKey([]byte("key"), key, input), 20)
	close(release)
	for i := 0; i < 20; i++ {
		if err = <-done; err != nil {
			t.Fatal(err)
		}
	}
	if physical.Load() != 1 || gate.admissions.Load() != 1 {
		t.Fatalf("physical=%d admissions=%d", physical.Load(), gate.admissions.Load())
	}
}

func TestRetriesExhaustExactlyFourPhysicalAdmissions(t *testing.T) {
	var calls atomic.Int32
	raw := application.NewFetchService(pipelineTransport(func(context.Context, domain.OutboundRequest) (domain.RawResponse, error) {
		calls.Add(1)
		return domain.RawResponse{StatusCode: 503}, nil
	}))
	gate := &countedGate{gate: readyGate(1)}
	raw.Gate = gate
	_, err := raw.Fetch(context.Background(), domain.FetchRequest{Policy: domain.FetchPolicy{MaxAttempts: 4, BaseDelay: time.Nanosecond, MaxBackoff: time.Nanosecond}})
	var failure *domain.UpstreamFailure
	if !errors.As(err, &failure) || failure.Attempts != 4 || !failure.RetryExhausted || !failure.Retryable || calls.Load() != 4 || gate.admissions.Load() != 4 {
		t.Fatalf("failure=%+v calls=%d admissions=%d", err, calls.Load(), gate.admissions.Load())
	}
}
