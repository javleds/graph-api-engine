package ports

import (
	"context"
	"encoding/json"
	"time"

	d "github.com/javleds/api-engine/internal/execution/domain"
)

type V2Store interface {
	ReadV2Input(context.Context, d.ExecutionID, string) (json.RawMessage, error)
	ReadV2State(context.Context, d.ExecutionID) (d.V2State, error)
	NextV2Task(context.Context, d.ExecutionID) (d.V2Task, error)
	ReadV2Task(context.Context, d.ExecutionID, string) (d.V2Task, error)
	SeedV2Tasks(context.Context, d.ExecutionID, string, int64, []d.V2Task) error
	CommitV2Task(context.Context, d.V2Commit) error
	ReserveV2Budget(context.Context, d.ExecutionID, int64, string, int64) error
	ReadV2Chunks(context.Context, string, d.ExecutionID, int64, int, int64, time.Time) (d.V2ChunkPage, error)
	ReadV2Tree(context.Context, string, d.ExecutionID, string, int64, int, time.Time) (d.V2TreePage, error)
}
