package bootstrap

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/application/ports"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"github.com/javleds/api-engine/internal/platform/observability"
	"time"
)

type measuredTransport struct {
	transport ports.OutboundTransport
	metrics   *observability.Metrics
}

func (t measuredTransport) Execute(ctx context.Context, input domain.OutboundRequest) (domain.RawResponse, error) {
	start := time.Now()
	response, err := t.transport.Execute(ctx, input)
	t.metrics.Observe("http_duration_seconds", time.Since(start).Seconds())
	if response.StatusCode == 429 {
		t.metrics.Add("upstream_rate_limited_total", 1)
	}
	return response, err
}
