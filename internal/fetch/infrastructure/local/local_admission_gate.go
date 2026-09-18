package local

import (
	"context"
	"errors"
	"github.com/javleds/api-engine/internal/fetch/application/ports"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"github.com/javleds/api-engine/internal/platform/clock"
	"math"
	"sync"
	"time"
)

type LocalAdmissionGate struct {
	mu      sync.Mutex
	states  map[string]*quotaState
	changed chan struct{}
	active  int
	options GateOptions
	store   ports.QuotaCooldownStore
	clock   clock.Clock
}

func NewLocalAdmissionGate(options GateOptions, store ports.QuotaCooldownStore, timer clock.Clock) *LocalAdmissionGate {
	if options.MaxConcurrent <= 0 {
		options.MaxConcurrent = 20
	}
	if options.MaxScopes <= 0 {
		options.MaxScopes = 4096
	}
	if options.Utilization <= 0 || options.Utilization > 1 {
		options.Utilization = .95
	}
	if timer == nil {
		timer = clock.RealClock{}
	}
	if options.ProcessStartedAt.IsZero() {
		options.ProcessStartedAt = timer.Now()
	}
	if options.AttemptTimeout <= 0 {
		options.AttemptTimeout = 30 * time.Second
	}
	return &LocalAdmissionGate{states: make(map[string]*quotaState), changed: make(chan struct{}), options: options, store: store, clock: timer}
}

func (g *LocalAdmissionGate) Acquire(ctx context.Context, request domain.AdmissionRequest) (ports.AdmissionPermit, error) {
	rules, err := uniqueRules(request.Rules)
	if err != nil {
		return nil, err
	}
	retained := make([]string, 0, len(rules))
	defer func() { g.releaseReferences(retained) }()
	for _, rule := range rules {
		if err = g.retainScope(ctx, rule); err != nil {
			return nil, err
		}
		retained = append(retained, rule.Scope)
	}
	for {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		g.mu.Lock()
		now := g.clock.Now()
		eligible, available, err := g.eligibility(rules, now)
		if err != nil {
			g.mu.Unlock()
			return nil, err
		}
		if available && !eligible.After(now) {
			permit := &localAdmissionPermit{gate: g}
			g.active++
			for _, rule := range rules {
				state := g.states[rule.Scope]
				state.active++
				state.lastUsed = now
				if state.rule.Requests > 0 {
					interval := time.Duration(math.Ceil(float64(state.rule.Period) / (float64(state.rule.Requests) * g.options.Utilization)))
					state.next = now.Add(interval)
				}
				permit.scopes = append(permit.scopes, rule.Scope)
			}
			g.mu.Unlock()
			return permit, nil
		}
		changed := g.changed
		g.mu.Unlock()
		deadline := request.Deadline
		if ctxDeadline, ok := ctx.Deadline(); ok && (deadline.IsZero() || ctxDeadline.Before(deadline)) {
			deadline = ctxDeadline
		}
		if !deadline.IsZero() && !eligible.Before(deadline) {
			return nil, &domain.UpstreamFailure{Code: "THROTTLE_WAIT_EXCEEDS_DEADLINE"}
		}
		wait := eligible.Sub(now)
		if wait <= 0 {
			wait = time.Hour
		}
		if !deadline.IsZero() && deadline.Sub(now) < wait {
			wait = deadline.Sub(now)
		}
		timer := g.clock.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-changed:
			timer.Stop()
		case <-timer.C():
		}
		if !deadline.IsZero() && !g.clock.Now().Before(deadline) {
			return nil, &domain.UpstreamFailure{Code: "THROTTLE_WAIT_EXCEEDS_DEADLINE"}
		}
	}
}

func (g *LocalAdmissionGate) eligibility(rules []domain.QuotaRule, now time.Time) (time.Time, bool, error) {
	eligible := now
	available := g.active < g.options.MaxConcurrent
	for _, rule := range rules {
		state := g.states[rule.Scope]
		if state.persistenceError != nil || state.loadError != nil {
			return now, false, errors.New("STORAGE_UNAVAILABLE")
		}
		if state.pendingWrites > 0 || (state.rule.Concurrency > 0 && state.active >= state.rule.Concurrency) {
			available = false
		}
		if state.next.After(eligible) {
			eligible = state.next
		}
		if state.cooldown.After(eligible) {
			eligible = state.cooldown
		}
	}
	return eligible, available, nil
}

func (g *LocalAdmissionGate) ensureScope(ctx context.Context, rule domain.QuotaRule) error {
	if err := g.retainScope(ctx, rule); err != nil {
		return err
	}
	g.releaseReferences([]string{rule.Scope})
	return nil
}

func (g *LocalAdmissionGate) retainScope(ctx context.Context, rule domain.QuotaRule) error {
	g.mu.Lock()
	if state, ok := g.states[rule.Scope]; ok {
		ready := state.ready
		state.references++
		g.mu.Unlock()
		select {
		case <-ctx.Done():
			g.releaseReferences([]string{rule.Scope})
			return ctx.Err()
		case <-ready:
		}
		g.mu.Lock()
		defer g.mu.Unlock()
		if state.loadError != nil {
			state.references--
			return errors.New("STORAGE_UNAVAILABLE")
		}
		declared := rule.Requests > 0 || rule.Concurrency > 0
		registered := state.rule.Requests > 0 || state.rule.Concurrency > 0
		if declared && registered && rule != state.rule {
			state.references--
			return &domain.UpstreamFailure{Code: "QUOTA_POLICY_CONFLICT"}
		}
		if declared && !registered {
			state.rule = rule
			g.setWarmup(state)
		}
		return nil
	}
	if len(g.states) >= g.options.MaxScopes {
		g.evictIdle()
	}
	if len(g.states) >= g.options.MaxScopes {
		g.mu.Unlock()
		return errors.New("FETCH_CAPACITY_EXCEEDED")
	}
	state := &quotaState{rule: rule, ready: make(chan struct{}), lastUsed: g.clock.Now(), references: 1}
	g.setWarmup(state)
	g.states[rule.Scope] = state
	g.mu.Unlock()
	var until time.Time
	var err error
	if g.store != nil {
		until, err = g.store.Get(ctx, rule.Scope)
	}
	g.mu.Lock()
	state.cooldown = until
	state.loadError = err
	if err != nil {
		state.references--
	}
	close(state.ready)
	g.signal()
	g.mu.Unlock()
	if err != nil {
		return errors.New("STORAGE_UNAVAILABLE")
	}
	return nil
}

func (g *LocalAdmissionGate) setWarmup(state *quotaState) {
	window := state.rule.Period
	if window < g.options.AttemptTimeout {
		window = g.options.AttemptTimeout
	}
	state.next = g.options.ProcessStartedAt.Add(window + g.options.SafetyMargin)
}

func (g *LocalAdmissionGate) Cooldown(ctx context.Context, scope string, until time.Time) error {
	if scope == "" {
		return errors.New("QUOTA_SCOPE_REQUIRED")
	}
	if err := g.retainScope(ctx, domain.QuotaRule{Scope: scope}); err != nil {
		return err
	}
	defer g.releaseReferences([]string{scope})
	g.mu.Lock()
	state := g.states[scope]
	if until.After(state.cooldown) {
		state.cooldown = until
	}
	state.pendingWrites++
	g.signal()
	g.mu.Unlock()
	var err error
	if g.store != nil {
		err = g.store.Extend(ctx, scope, until)
	}
	g.mu.Lock()
	state.pendingWrites--
	state.persistenceError = err
	g.signal()
	g.mu.Unlock()
	if err != nil {
		return errors.New("STORAGE_UNAVAILABLE")
	}
	return nil
}

func (g *LocalAdmissionGate) signal() { close(g.changed); g.changed = make(chan struct{}) }

func uniqueRules(input []domain.QuotaRule) ([]domain.QuotaRule, error) {
	rules := make([]domain.QuotaRule, 0, len(input))
	seen := map[string]domain.QuotaRule{}
	for _, rule := range input {
		if rule.Scope == "" || rule.Requests < 0 || rule.Concurrency < 0 || (rule.Requests > 0 && rule.Period <= 0) {
			return nil, errors.New("invalid quota rule")
		}
		if prior, ok := seen[rule.Scope]; ok {
			if prior != rule {
				return nil, &domain.UpstreamFailure{Code: "QUOTA_POLICY_CONFLICT"}
			}
			continue
		}
		seen[rule.Scope] = rule
		rules = append(rules, rule)
	}
	return rules, nil
}

// Register validates shared policies without consuming pacing or concurrency.
func (g *LocalAdmissionGate) Register(ctx context.Context, rules []domain.QuotaRule) error {
	unique, err := uniqueRules(rules)
	if err != nil {
		return err
	}
	for _, rule := range unique {
		if err = g.ensureScope(ctx, rule); err != nil {
			return err
		}
	}
	return nil
}

func (g *LocalAdmissionGate) releaseReferences(scopes []string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, scope := range scopes {
		g.states[scope].references--
	}
}

func (g *LocalAdmissionGate) evictIdle() {
	now := g.clock.Now()
	for scope, state := range g.states {
		retention := state.rule.Period
		if retention < g.options.AttemptTimeout {
			retention = g.options.AttemptTimeout
		}
		if state.references == 0 && state.active == 0 && state.pendingWrites == 0 && state.loadError == nil && state.persistenceError == nil && !state.cooldown.After(now) && !state.next.After(now) && now.Sub(state.lastUsed) >= retention {
			delete(g.states, scope)
		}
	}
}
