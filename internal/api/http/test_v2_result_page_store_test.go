package http_test

import (
	"context"
	"time"

	"github.com/javleds/api-engine/internal/execution/application/ports"
	"github.com/javleds/api-engine/internal/execution/domain"
)

type testV2ResultPageStore struct {
	ports.V2Store
	ports.V2ResultStore
	page domain.V2ResultPage
}

func (s *testV2ResultPageStore) ReadV2Results(context.Context, string, domain.ExecutionID, int64, int64, int, time.Time) (domain.V2ResultPage, error) {
	return s.page, nil
}
