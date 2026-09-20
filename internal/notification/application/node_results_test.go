package application

import (
	"context"
	"encoding/json"
	executionports "github.com/javleds/api-engine/internal/execution/application/ports"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	"io"
	"time"
)

type nodeResults struct {
	executionports.V2ResultStore
	event  execution.V2Result
	data   string
	writes int
}

func (r *nodeResults) ReadV2Results(_ context.Context, _ string, _ execution.ExecutionID, after, _ int64, _ int, _ time.Time) (execution.V2ResultPage, error) {
	page := execution.V2ResultPage{}
	if after == 0 {
		page.Results = []execution.V2Result{r.event}
	}
	return page, nil
}
func (r *nodeResults) WriteV2Result(_ context.Context, _ string, _ execution.ExecutionID, _ string, _ time.Time, w io.Writer) error {
	r.writes++
	raw, _ := json.Marshal(r.event)
	if _, err := w.Write(raw[:len(raw)-1]); err != nil {
		return err
	}
	_, err := io.WriteString(w, `,"data":`+r.data+`,"errors":[],"complete":true}`)
	return err
}
