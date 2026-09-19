package application

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/javleds/api-engine/internal/execution/application/ports"
	"github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/platform/clock"
	"github.com/javleds/api-engine/internal/platform/identifier"
)

type CreateExecution struct {
	OnCreated         func(domain.Execution)
	Store             ports.ExecutionStore
	Dispatcher        ports.ExecutionDispatcher
	Finalizer         ports.ExecutionFinalizer
	IDs               identifier.Generator
	Clock             clock.Clock
	IdentityKey       []byte
	MetadataRetention time.Duration
	ExecutionTimeout  time.Duration
	CheckCapacity     func(context.Context) error
}

func (c *CreateExecution) Execute(ctx context.Context, command CreateExecutionCommand) (CreationResult, error) {
	version := command.APIVersion
	if version == 0 {
		version = 1
	}
	if version != 1 && version != 2 {
		return CreationResult{}, errors.New("unsupported execution protocol")
	}
	purpose := ""
	if version == 2 {
		purpose = ":v2"
	}
	fingerprint := c.identity("request"+purpose, string(command.Definition))
	key := ""
	if command.IdempotencyKey != "" {
		key = c.identity("idempotency"+purpose, command.OwnerID+"\x00"+command.IdempotencyKey)
		existing, err := c.Store.FindReplay(ctx, command.OwnerID, key)
		if err == nil {
			if existing.Fingerprint != fingerprint {
				return CreationResult{}, domain.ErrIdempotencyConflict
			}
			return CreationResult{Execution: existing, Replay: true}, nil
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return CreationResult{}, err
		}
	}
	if c.CheckCapacity != nil {
		if err := c.CheckCapacity(ctx); err != nil {
			return c.replayAfterRejection(ctx, command.OwnerID, key, fingerprint, err)
		}
	}
	reservation, err := c.Dispatcher.Reserve(ctx)
	if err != nil {
		return c.replayAfterRejection(ctx, command.OwnerID, key, fingerprint, err)
	}
	defer reservation.Cancel()
	id, err := c.IDs.NewID()
	if err != nil {
		return CreationResult{}, fmt.Errorf("generate execution identity: %w", err)
	}
	now := c.Clock.Now()
	timeout := c.ExecutionTimeout
	if timeout <= 0 {
		timeout = 6 * time.Hour
	}
	execution, replay, err := c.Store.Create(ctx, domain.ExecutionSnapshot{
		APIVersion: version, DeadlineAt: now.Add(timeout),
		ID: domain.ExecutionID(id), OwnerID: command.OwnerID, Fingerprint: fingerprint, IdempotencyKeyHash: key,
		Definition: command.Definition, WebhookEnabled: command.WebhookEnabled, CreatedAt: now, MetadataExpiresAt: now.Add(c.MetadataRetention),
	})
	if err != nil {
		return CreationResult{}, err
	}
	result := CreationResult{Execution: execution, Replay: replay}
	if replay {
		return result, nil
	}
	if err := reservation.Commit(ctx, execution.ID); err != nil {
		persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		failure := &domain.ExecutionFailure{Code: "DISPATCH_FAILED", Message: "The persisted execution could not be dispatched."}
		finishErr := c.Finalizer.Finish(persistCtx, domain.ExecutionCompletion{ID: execution.ID, ExpectedVersion: execution.Version, Status: domain.ExecutionStatus("failed"), Error: failure, FinishedAt: c.Clock.Now()})
		if finishErr != nil {
			return result, fmt.Errorf("dispatch and terminal persistence failed: %w", finishErr)
		}
		return result, domain.StorageError("DISPATCH_FAILED")
	}
	if c.OnCreated != nil {
		c.OnCreated(execution)
	}
	return result, nil
}

func (c *CreateExecution) identity(purpose, value string) string {
	mac := hmac.New(sha256.New, c.IdentityKey)
	mac.Write([]byte(purpose + "\x00" + value))
	return hex.EncodeToString(mac.Sum(nil))
}

// Recheck after admission rejects: a competing request may have just committed.
func (c *CreateExecution) replayAfterRejection(ctx context.Context, owner, key, fingerprint string, rejection error) (CreationResult, error) {
	if key == "" {
		return CreationResult{}, rejection
	}
	existing, err := c.Store.FindReplay(ctx, owner, key)
	if errors.Is(err, domain.ErrNotFound) {
		return CreationResult{}, rejection
	}
	if err != nil {
		return CreationResult{}, err
	}
	if existing.Fingerprint != fingerprint {
		return CreationResult{}, domain.ErrIdempotencyConflict
	}
	return CreationResult{Execution: existing, Replay: true}, nil
}
