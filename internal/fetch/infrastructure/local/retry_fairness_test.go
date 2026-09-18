package local

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/application"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"runtime"
	"testing"
	"time"
)

func TestBackoffReleasesGlobalSlotForIndependentProvider(t *testing.T) {
	timer := &manualClock{now: time.Now()}
	gate := NewLocalAdmissionGate(GateOptions{MaxConcurrent: 1, ProcessStartedAt: timer.Now().Add(-time.Hour)}, nil, timer)
	raw := application.NewFetchService(pipelineTransport(func(_ context.Context, r domain.OutboundRequest) (domain.RawResponse, error) {
		if r.URL == "https://slow.example" {
			return domain.RawResponse{StatusCode: 503, Headers: map[string][]string{"Retry-After": {"17"}}}, nil
		}
		return domain.RawResponse{StatusCode: 200, Body: []byte(`[]`)}, nil
	}))
	raw.Gate = gate
	raw.Clock = timer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	slow := make(chan error, 1)
	go func() {
		_, err := raw.Fetch(ctx, domain.FetchRequest{Request: domain.OutboundRequest{URL: "https://slow.example"}, Admission: domain.AdmissionRequest{ProviderScope: "slow", Rules: []domain.QuotaRule{{Scope: "slow", Concurrency: 1}}}})
		slow <- err
	}()
	deadline := time.Now().Add(time.Second)
	for {
		timer.mu.Lock()
		waiting := len(timer.timers) > 0
		timer.mu.Unlock()
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("retry did not enter backoff")
		}
		runtime.Gosched()
	}
	independentCtx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	_, err := raw.Fetch(independentCtx, domain.FetchRequest{Request: domain.OutboundRequest{URL: "https://independent.example"}, Admission: domain.AdmissionRequest{ProviderScope: "other", Rules: []domain.QuotaRule{{Scope: "other", Concurrency: 1}}}})
	if err != nil {
		t.Fatalf("independent provider blocked: %v", err)
	}
	gate.mu.Lock()
	active := gate.active
	gate.mu.Unlock()
	if active != 0 {
		t.Fatalf("backoff retains %d permits", active)
	}
	cancel()
	<-slow
}
