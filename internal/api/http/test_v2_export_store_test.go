package http_test

import (
	"context"
	"github.com/javleds/api-engine/internal/execution/application/ports"
	"github.com/javleds/api-engine/internal/execution/domain"
	"time"
)

type testV2ExportStore struct {
	ports.V2Store
	afterFirstRead func()
}

func (s *testV2ExportStore) ReadV2Chunks(ctx context.Context, owner string, id domain.ExecutionID, after int64, limit int, maxBytes int64, now time.Time) (domain.V2ChunkPage, error) {
	page, err := s.V2Store.ReadV2Chunks(ctx, owner, id, after, limit, maxBytes, now)
	if s.afterFirstRead != nil {
		callback := s.afterFirstRead
		s.afterFirstRead = nil
		callback()
	}
	return page, err
}
