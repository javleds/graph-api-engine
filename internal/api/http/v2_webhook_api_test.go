package http_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	api "github.com/javleds/api-engine/internal/api/http"
	"github.com/javleds/api-engine/internal/api/http/dto"
	"github.com/javleds/api-engine/internal/execution/domain"
	notification "github.com/javleds/api-engine/internal/notification/domain"
	"github.com/javleds/api-engine/internal/platform/clock"
)

type testNodeWebhookStatus struct {
	reads int
}

func (s *testNodeWebhookStatus) Status(context.Context, string, domain.ExecutionID) (*notification.WebhookStatus, error) {
	return &notification.WebhookStatus{Status: "delivered", Attempts: 1}, nil
}

func (s *testNodeWebhookStatus) NodeSummary(context.Context, string, domain.ExecutionID) (notification.NodeWebhookSummary, error) {
	return notification.NodeWebhookSummary{Pending: 1, Failed: 2}, nil
}

func (s *testNodeWebhookStatus) DeliveryPage(_ context.Context, _ string, _ domain.ExecutionID, after string, limit int) (notification.WebhookDeliveryPage, error) {
	s.reads++
	return notification.WebhookDeliveryPage{Deliveries: []notification.WebhookDeliveryStatus{{EventID: "event", NodeID: "node", Sequence: "1", Status: "failed", Error: &domain.ExecutionFailure{Code: "WEBHOOK_PAYLOAD_TOO_LARGE"}}}, NextCursor: after, HasMore: limit == 1}, nil
}

func TestV2WebhookReadsIsolateOwnerAndKeepTerminalStatus(t *testing.T) {
	h, store, _ := newTestAPI(t)
	withWebhook := append(append([]byte{}, v2Request[:len(v2Request)-1]...), []byte(`,"webhook":{"url":"https://example.com/hook","payload":"status"}}`)...)
	accepted := request(t, h, "POST", "/v2/executions", withWebhook, "", "owner")
	if accepted.Code != 202 {
		t.Fatalf("%d %s", accepted.Code, accepted.Body)
	}
	var execution dto.CreateExecutionResponse
	if err := json.Unmarshal(accepted.Body.Bytes(), &execution); err != nil {
		t.Fatal(err)
	}
	reader := &testNodeWebhookStatus{}
	server := &api.Server{Reader: store, V2Store: store, WebhookStatus: reader, Auth: testAuthenticator{}, Clock: clock.RealClock{}}
	h = server.Handler()
	base := "/v2/executions/" + string(execution.ID)
	status := request(t, h, "GET", base, nil, "", "owner")
	if status.Code != 200 || !strings.Contains(status.Body.String(), `"webhook":{"status":"delivered","attempts":1}`) || !strings.Contains(status.Body.String(), `"nodeWebhookSummary":{"pending":1,"delivered":0,"failed":2}`) {
		t.Fatalf("status: %d %s", status.Code, status.Body)
	}
	page := request(t, h, "GET", base+"/webhooks?after=0&limit=1", nil, "", "owner")
	if page.Code != 200 || !strings.Contains(page.Body.String(), `WEBHOOK_PAYLOAD_TOO_LARGE`) || !strings.Contains(page.Body.String(), `"hasMore":true`) {
		t.Fatalf("deliveries: %d %s", page.Code, page.Body)
	}
	denied := request(t, h, "GET", base+"/webhooks", nil, "", "other")
	if denied.Code != 404 || reader.reads != 1 {
		t.Fatalf("owner isolation: %d reads=%d", denied.Code, reader.reads)
	}
	invalid := request(t, h, "GET", base+"/webhooks?after=01", nil, "", "owner")
	if invalid.Code != 422 || reader.reads != 1 {
		t.Fatalf("cursor: %d reads=%d", invalid.Code, reader.reads)
	}
}
