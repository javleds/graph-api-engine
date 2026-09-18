package local

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"runtime"
	"testing"
	"time"
)

func TestCooldownReadFailureCanRecoverWithoutRestart(t *testing.T) {
	store := &initializationStore{failFirst: true}
	gate := NewLocalAdmissionGate(GateOptions{ProcessStartedAt: time.Now().Add(-time.Hour)}, store, nil)
	rules := []domain.QuotaRule{{Scope: "provider", Requests: 10, Period: time.Minute}}
	if err := gate.Register(context.Background(), rules); err == nil {
		t.Fatal("failed read allowed registration")
	}
	if err := gate.Register(context.Background(), rules); err != nil {
		t.Fatalf("healthy retry stayed blocked: %v", err)
	}
	if store.readCount() != 2 {
		t.Fatalf("reads=%d", store.readCount())
	}
	permit, err := gate.Acquire(context.Background(), domain.AdmissionRequest{Rules: rules})
	if err != nil {
		t.Fatal(err)
	}
	_ = permit.Release(context.Background())
}

func TestCanceledInitializerDoesNotPoisonConcurrentWaiters(t *testing.T) {
	store := &initializationStore{entered: make(chan struct{}), release: make(chan struct{})}
	gate := NewLocalAdmissionGate(GateOptions{ProcessStartedAt: time.Now().Add(-time.Hour)}, store, nil)
	rules := []domain.QuotaRule{{Scope: "provider", Concurrency: 1}}
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() { first <- gate.Register(ctx, rules) }()
	<-store.entered
	waiters := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() { waiters <- gate.Register(context.Background(), rules) }()
	}
	awaitQuotaReferences(t, gate, "provider", 9)
	cancel()
	if err := <-first; err == nil {
		t.Fatal("canceled load succeeded")
	}
	retry := make(chan error, 1)
	go func() { retry <- gate.Register(context.Background(), rules) }()
	for i := 0; i < 8; i++ {
		if err := <-waiters; err == nil {
			t.Fatal("failed initialization waiter admitted")
		}
	}
	if err := <-retry; err != nil {
		t.Fatalf("fresh retry failed: %v", err)
	}
	if store.readCount() != 2 {
		t.Fatalf("expected one coordinated reload, got %d reads", store.readCount())
	}
	gate.mu.Lock()
	references := gate.states["provider"].references
	active := gate.active
	gate.mu.Unlock()
	if references != 0 || active != 0 {
		t.Fatalf("references=%d active=%d", references, active)
	}
}

func TestRetryRestoresCooldownWithoutResettingWarmup(t *testing.T) {
	now := time.Now()
	store := &initializationStore{failFirst: true, until: now.Add(time.Hour)}
	gate := NewLocalAdmissionGate(GateOptions{ProcessStartedAt: now}, store, nil)
	rules := []domain.QuotaRule{{Scope: "provider", Requests: 1, Period: time.Minute}}
	if err := gate.Register(context.Background(), rules); err == nil {
		t.Fatal("failed initial read accepted")
	}
	gate.mu.Lock()
	warmup := gate.states["provider"].next
	gate.mu.Unlock()
	if err := gate.Register(context.Background(), rules); err != nil {
		t.Fatal(err)
	}
	gate.mu.Lock()
	after := gate.states["provider"].next
	gate.mu.Unlock()
	if !warmup.Equal(after) {
		t.Fatal("reloading reset pacing state")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := gate.Acquire(ctx, domain.AdmissionRequest{Rules: rules}); err == nil || err.Error() != "THROTTLE_WAIT_EXCEEDS_DEADLINE" {
		t.Fatalf("durable cooldown bypassed: %v", err)
	}
}

func awaitQuotaReferences(t *testing.T, gate *LocalAdmissionGate, scope string, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		gate.mu.Lock()
		count := gate.states[scope].references
		gate.mu.Unlock()
		if count == want {
			return
		}
		runtime.Gosched()
	}
	t.Fatal("quota waiters did not attach")
}
