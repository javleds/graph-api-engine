package local

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"runtime"
	"sync"
	"testing"
	"time"
)

func readyGate(maximum int) *LocalAdmissionGate {
	return NewLocalAdmissionGate(GateOptions{MaxConcurrent: maximum, ProcessStartedAt: time.Now().Add(-time.Hour), Utilization: 1}, nil, nil)
}

func TestGateCancellationAndIdempotentRelease(t *testing.T) {
	gate := readyGate(1)
	request := domain.AdmissionRequest{Rules: []domain.QuotaRule{{Scope: "shared", Concurrency: 1}}}
	permit, err := gate.Acquire(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = gate.Acquire(ctx, request); err == nil {
		t.Fatal("canceled acquire succeeded")
	}
	_ = permit.Release(context.Background())
	_ = permit.Release(context.Background())
	if gate.active != 0 || gate.states["shared"].active != 0 {
		t.Fatal("release corrupted counters")
	}
}

func TestGateGlobalConcurrency(t *testing.T) {
	gate := readyGate(2)
	var mu sync.Mutex
	active, peak := 0, 0
	var wait sync.WaitGroup
	for i := 0; i < 30; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			permit, err := gate.Acquire(context.Background(), domain.AdmissionRequest{})
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			active++
			if active > peak {
				peak = active
			}
			mu.Unlock()
			mu.Lock()
			active--
			mu.Unlock()
			_ = permit.Release(context.Background())
		}()
	}
	wait.Wait()
	if peak > 2 || gate.active != 0 {
		t.Fatalf("peak %d active %d", peak, gate.active)
	}
}

func TestGateConflictAndRegisteredOmission(t *testing.T) {
	gate := readyGate(10)
	rule := domain.QuotaRule{Scope: "shared", Requests: 1, Period: time.Second, Concurrency: 1}
	permit, err := gate.Acquire(context.Background(), domain.AdmissionRequest{Rules: []domain.QuotaRule{rule}})
	if err != nil {
		t.Fatal(err)
	}
	_ = permit.Release(context.Background())
	conflict := rule
	conflict.Requests = 2
	if _, err = gate.Acquire(context.Background(), domain.AdmissionRequest{Rules: []domain.QuotaRule{conflict}}); err == nil || err.Error() != "QUOTA_POLICY_CONFLICT" {
		t.Fatalf("%v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err = gate.Acquire(ctx, domain.AdmissionRequest{Rules: []domain.QuotaRule{{Scope: "shared"}}}); err == nil {
		t.Fatal("omission bypassed registered policy")
	}
}

func TestGateBlockedEndpointDoesNotChargeProvider(t *testing.T) {
	gate := readyGate(10)
	provider := domain.QuotaRule{Scope: "provider", Requests: 100, Period: time.Second}
	endpoint := domain.QuotaRule{Scope: "endpoint", Requests: 1, Period: time.Second}
	if err := gate.ensureScope(context.Background(), provider); err != nil {
		t.Fatal(err)
	}
	before := gate.states[provider.Scope].next
	if err := gate.Cooldown(context.Background(), endpoint.Scope, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := gate.Acquire(ctx, domain.AdmissionRequest{Rules: []domain.QuotaRule{provider, endpoint}})
	if err == nil || !gate.states[provider.Scope].next.Equal(before) || gate.active != 0 {
		t.Fatal("partial quota consumed")
	}
}

func TestGateColdStartRespectsWindow(t *testing.T) {
	gate := NewLocalAdmissionGate(GateOptions{}, nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err := gate.Acquire(ctx, domain.AdmissionRequest{Rules: []domain.QuotaRule{{Scope: "provider", Requests: 100, Period: time.Minute}}})
	if err == nil || err.Error() != "THROTTLE_WAIT_EXCEEDS_DEADLINE" {
		t.Fatalf("%v", err)
	}
}

func TestGatePacesJointScopesWithControlledClock(t *testing.T) {
	timer := &manualClock{now: time.Now()}
	gate := NewLocalAdmissionGate(GateOptions{MaxConcurrent: 10, Utilization: 1, ProcessStartedAt: timer.Now().Add(-time.Hour)}, nil, timer)
	request := domain.AdmissionRequest{Rules: []domain.QuotaRule{{Scope: "provider", Requests: 100, Period: time.Minute}, {Scope: "endpoint", Requests: 20, Period: time.Minute}}}
	permit, err := gate.Acquire(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	_ = permit.Release(context.Background())
	second := make(chan error, 1)
	go func() {
		permit, err := gate.Acquire(context.Background(), request)
		if err == nil {
			_ = permit.Release(context.Background())
		}
		second <- err
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
			t.Fatal("gate did not start timer")
		}
		runtime.Gosched()
	}
	before := gate.states["provider"].next
	timer.Advance(2 * time.Second)
	select {
	case <-second:
		t.Fatal("endpoint pacing bypassed")
	default:
	}
	gate.mu.Lock()
	charged := !gate.states["provider"].next.Equal(before)
	gate.mu.Unlock()
	if charged {
		t.Fatal("provider charged before endpoint")
	}
	timer.Advance(time.Second)
	select {
	case err := <-second:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("admission did not resume")
	}
}

func TestGateRegisterDoesNotChargeAndEvictsOnlyIdle(t *testing.T) {
	timer := &manualClock{now: time.Now()}
	gate := NewLocalAdmissionGate(GateOptions{MaxScopes: 1, AttemptTimeout: time.Second, ProcessStartedAt: timer.Now().Add(-time.Hour)}, nil, timer)
	rule := domain.QuotaRule{Scope: "one", Requests: 1, Period: time.Second}
	if err := gate.Register(context.Background(), []domain.QuotaRule{rule}); err != nil {
		t.Fatal(err)
	}
	if gate.active != 0 || gate.states["one"].active != 0 {
		t.Fatal("registration consumed concurrency")
	}
	conflict := rule
	conflict.Requests = 2
	if err := gate.Register(context.Background(), []domain.QuotaRule{conflict}); err == nil || err.Error() != "QUOTA_POLICY_CONFLICT" {
		t.Fatalf("%v", err)
	}
	permit, err := gate.Acquire(context.Background(), domain.AdmissionRequest{Rules: []domain.QuotaRule{rule}})
	if err != nil {
		t.Fatal(err)
	}
	timer.Advance(2 * time.Second)
	if err = gate.Register(context.Background(), []domain.QuotaRule{{Scope: "two"}}); err == nil {
		t.Fatal("active scope evicted")
	}
	_ = permit.Release(context.Background())
	if err = gate.Register(context.Background(), []domain.QuotaRule{{Scope: "two"}}); err != nil {
		t.Fatal(err)
	}
	if gate.states["one"] != nil || gate.states["two"] == nil {
		t.Fatal("idle scope not reclaimed")
	}
}
