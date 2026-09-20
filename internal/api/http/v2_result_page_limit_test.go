package http_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	api "github.com/javleds/api-engine/internal/api/http"
	"github.com/javleds/api-engine/internal/api/http/dto"
	"github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/platform/clock"
)

func TestV2ResultPageIncludesLinksWithinResponseBudget(t *testing.T) {
	store := &testV2ResultPageStore{page: domain.V2ResultPage{NextCursor: "100", LastSequence: "100"}}
	for i := 1; i <= 100; i++ {
		sequence := strconv.Itoa(i)
		store.page.Results = append(store.page.Results, domain.V2Result{NodeID: sequence, Sequence: sequence, Root: strings.Repeat("x", 10500)})
	}
	server := &api.Server{V2Store: store, Auth: testAuthenticator{}, Clock: clock.RealClock{}}
	response := request(t, server.Handler(), "GET", "/v2/executions/example/results", nil, "", "owner")
	var page dto.V2ResultPage
	if response.Code != 200 || response.Body.Len() > 1<<20 || json.Unmarshal(response.Body.Bytes(), &page) != nil || !page.HasMore || len(page.Results) >= 100 || len(page.Results) == 0 {
		t.Fatalf("unbounded page: status=%d bytes=%d", response.Code, response.Body.Len())
	}
	last := page.Results[len(page.Results)-1]
	if page.NextCursor != last.Sequence || last.Links.Result != "/v2/executions/example/results/"+last.NodeID {
		t.Fatalf("invalid trimmed cursor/link: %+v", last)
	}
}
