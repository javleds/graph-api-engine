package application

import (
	"context"
	"errors"
	"github.com/javleds/api-engine/internal/fetch/application/ports"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"github.com/javleds/api-engine/internal/platform/clock"
	"math/rand/v2"
	"net/http"
	"time"
)

type FetchService struct {
	MaxJSONDepth  int
	MaxJSONValues int
	Transport     ports.OutboundTransport
	Gate          ports.AdmissionGate
	Clock         clock.Clock
	BeforeAttempt func(context.Context) error
	OnAttempt     func()
}

func NewFetchService(transport ports.OutboundTransport) *FetchService {
	return &FetchService{Transport: transport, Clock: clock.RealClock{}, MaxJSONDepth: 64, MaxJSONValues: 100000}
}

func (s *FetchService) Fetch(ctx context.Context, input domain.FetchRequest) (domain.FetchResult, error) {
	input.Policy = input.Policy.WithDefaults()
	ctx, cancel := context.WithTimeout(ctx, input.Policy.OperationTimeout)
	defer cancel()
	input.Admission.Deadline, _ = ctx.Deadline()
	response, err := s.fetchPhysical(ctx, input)
	return domain.FetchResult{Response: response}, err
}

func (s *FetchService) fetchPhysical(ctx context.Context, input domain.FetchRequest) (domain.RawResponse, error) {
	policy := input.Policy
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return domain.RawResponse{}, err
		}
		response, failure, wait, err := s.attempt(ctx, input, attempt)
		if err != nil {
			return domain.RawResponse{}, err
		}
		if failure == nil {
			response.Attempts = attempt
			return response, nil
		}
		failure.Attempts = attempt
		failure.RetryExhausted = failure.Retryable && attempt == policy.MaxAttempts
		if !failure.Retryable || failure.RetryExhausted {
			return domain.RawResponse{}, failure
		}
		if deadline, ok := ctx.Deadline(); ok && !s.Clock.Now().Add(wait).Before(deadline) {
			return domain.RawResponse{}, &domain.UpstreamFailure{Code: "THROTTLE_WAIT_EXCEEDS_DEADLINE", HTTPStatus: failure.HTTPStatus, Attempts: attempt, Retryable: failure.Retryable}
		}
		timer := s.Clock.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return domain.RawResponse{}, ctx.Err()
		case <-timer.C():
		}
	}
	return domain.RawResponse{}, errors.New("invalid retry policy")
}

func (s *FetchService) attempt(ctx context.Context, input domain.FetchRequest, attempt int) (response domain.RawResponse, failure *domain.UpstreamFailure, wait time.Duration, err error) {
	var permit ports.AdmissionPermit
	if s.Gate != nil {
		permit, err = s.Gate.Acquire(ctx, input.Admission)
		if err != nil {
			return
		}
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			failure = &domain.UpstreamFailure{Code: "INTERNAL_WORKER_PANIC"}
			err = nil
		}
		if permit != nil {
			if releaseErr := permit.Release(context.WithoutCancel(ctx)); releaseErr != nil && err == nil {
				err = releaseErr
			}
		}
	}()
	if s.BeforeAttempt != nil {
		if err = s.BeforeAttempt(ctx); err != nil {
			return
		}
	}
	if input.BeforeAttempt != nil {
		if err = input.BeforeAttempt(ctx); err != nil {
			return
		}
	}
	attemptCtx, cancel := context.WithTimeout(ctx, input.Policy.AttemptTimeout)
	defer cancel()
	if s.OnAttempt != nil {
		s.OnAttempt()
	}
	if input.OnAttempt != nil {
		input.OnAttempt()
	}
	response, err = s.Transport.Execute(attemptCtx, input.Request)
	if ctx.Err() != nil {
		err = ctx.Err()
		return
	}
	failure = classify(response, err)
	err = nil
	if failure == nil {
		depth, values := s.MaxJSONDepth, s.MaxJSONValues
		if depth <= 0 {
			depth = 64
		}
		if values <= 0 {
			values = 100000
		}
		failure = validateJSONLimits(response.Body, depth, values)
		return
	}
	wait = backoff(input.Policy, attempt)
	if response.StatusCode == 429 || response.StatusCode == 503 {
		retryAfter, valid := RetryAfter(http.Header(response.Headers).Get("Retry-After"), s.Clock.Now())
		if valid && retryAfter > wait {
			wait = retryAfter
		}
		if s.Gate != nil && (response.StatusCode == 429 || valid) {
			scopes := append([]string{input.Admission.ProviderScope}, input.Admission.CooldownScopes...)
			for _, scope := range scopes {
				cooldownErr := s.Gate.Cooldown(ctx, scope, s.Clock.Now().Add(wait))
				if cooldownErr != nil {
					err = cooldownErr
				}
			}
		}
	}
	return
}

func backoff(policy domain.FetchPolicy, attempt int) time.Duration {
	cap := policy.BaseDelay
	for i := 1; i < attempt && cap < policy.MaxBackoff; i++ {
		if cap > policy.MaxBackoff/2 {
			cap = policy.MaxBackoff
			break
		}
		cap *= 2
	}
	if cap > policy.MaxBackoff {
		cap = policy.MaxBackoff
	}
	if cap <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(cap)))
}
