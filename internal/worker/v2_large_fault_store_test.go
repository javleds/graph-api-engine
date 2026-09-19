package worker

import (
	"context"
	"errors"
	"github.com/javleds/api-engine/internal/execution/application/ports"
	execution "github.com/javleds/api-engine/internal/execution/domain"
)

// v2LargeFaultStore simulates losing a commit acknowledgment after SQLite has
// durably saved it. Recovery must trust storage rather than replay accepted HTTP.
type v2LargeFaultStore struct {
	ports.V2Store
	published      int64
	interruptAfter int64
}

func (s *v2LargeFaultStore) CommitV2Task(ctx context.Context, commit execution.V2Commit) error {
	if err := s.V2Store.CommitV2Task(ctx, commit); err != nil {
		return err
	}
	for _, chunk := range commit.Chunks {
		s.published += int64(len(chunk.Value))
	}
	if s.interruptAfter > 0 && s.published >= s.interruptAfter {
		s.interruptAfter = 0
		return errors.New("simulated lost commit acknowledgment")
	}
	return nil
}
