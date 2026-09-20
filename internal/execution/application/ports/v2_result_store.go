package ports

import (
	"context"
	d "github.com/javleds/api-engine/internal/execution/domain"
	"io"
	"time"
)

type V2ResultStore interface {
	ReadV2Results(context.Context, string, d.ExecutionID, int64, int64, int, time.Time) (d.V2ResultPage, error)
	ReadV2Result(context.Context, string, d.ExecutionID, string, time.Time) (d.V2Result, error)
	WriteV2Result(context.Context, string, d.ExecutionID, string, time.Time, io.Writer) error
}
