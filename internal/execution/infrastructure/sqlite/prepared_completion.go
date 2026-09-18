package sqlite

import (
	"context"
	"encoding/json"
	d "github.com/javleds/api-engine/internal/execution/domain"
)

type preparedCompletion struct {
	failure  any
	report   string
	progress string
	expires  any
	result   []byte
	target   []byte
	payload  []byte
}

func prepareCompletion(ctx context.Context, s *ExecutionStore, owner string, c d.ExecutionCompletion) (preparedCompletion, error) {
	p := preparedCompletion{}
	report, err := json.Marshal(c.Report)
	if err != nil {
		return p, err
	}
	p.report = string(report)
	progress, err := json.Marshal(c.Progress)
	if err != nil {
		return p, err
	}
	p.progress = string(progress)
	if c.Error != nil {
		failure, err := json.Marshal(c.Error)
		if err != nil {
			return p, err
		}
		p.failure = string(failure)
	}
	if c.Status != d.Failed {
		p.expires = c.ResultExpiresAt.UnixMilli()
		p.result, err = s.seal(ctx, "execution_results", owner, string(c.ID), c.Data)
		if err != nil {
			return p, err
		}
	}
	if c.Delivery != nil {
		p.target, err = s.seal(ctx, "webhook_target", owner, c.Delivery.ID, c.Delivery.Target)
		if err != nil {
			return p, err
		}
		p.payload, err = s.seal(ctx, "webhook_payload", owner, c.Delivery.ID, c.Delivery.Payload)
		if err != nil {
			return p, err
		}
	}
	return p, nil
}
