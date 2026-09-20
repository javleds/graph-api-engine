package http_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/javleds/api-engine/internal/api/http/dto"
	"github.com/javleds/api-engine/internal/execution/domain"
)

func TestV2CompleteResultsAPI(t *testing.T) {
	h, store, _ := newTestAPI(t)
	accepted := request(t, h, "POST", "/v2/executions", v2Request, "", "owner")
	if accepted.Code != 202 {
		t.Fatalf("%d %s", accepted.Code, accepted.Body)
	}
	var execution dto.CreateExecutionResponse
	if err := json.Unmarshal(accepted.Body.Bytes(), &execution); err != nil {
		t.Fatal(err)
	}
	base := "/v2/executions/" + string(execution.ID)
	if execution.Links.Results != base+"/results" {
		t.Fatalf("missing results link: %+v", execution.Links)
	}
	state, err := store.ReadV2State(context.Background(), execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	commit := func(chunks []domain.V2Chunk, results []domain.V2Result) {
		t.Helper()
		if err := store.CommitV2Task(context.Background(), domain.V2Commit{ID: execution.ID, OwnerID: "owner", Generation: state.Generation, Chunks: chunks, Results: results}); err != nil {
			t.Fatal(err)
		}
	}
	commit([]domain.V2Chunk{
		{Type: "data", Operation: "object", NodeID: "bulbasaur", Key: "pokemon"},
		{Type: "data", Operation: "value", NodeID: "number", ParentID: "bulbasaur", Key: "id", Value: json.RawMessage(`9007199254740993`)},
	}, nil)
	page := request(t, h, "GET", base+"/results", nil, "", "owner")
	if page.Code != 200 || !strings.Contains(page.Body.String(), `"results":[]`) {
		t.Fatalf("unfinished chain published: %d %s", page.Code, page.Body)
	}
	commit([]domain.V2Chunk{{Type: "data", Operation: "close", NodeID: "bulbasaur", Status: "completed"}}, []domain.V2Result{{NodeID: "bulbasaur", Root: "pokemon", Status: "completed", FirstChunk: 1, LastChunk: 3}})
	commit([]domain.V2Chunk{{Type: "data", Operation: "value", NodeID: "language", Key: "language", Value: json.RawMessage(`"en"`)}}, []domain.V2Result{{NodeID: "language", Root: "language", Status: "completed", FirstChunk: 4, LastChunk: 4}})
	page = request(t, h, "GET", base+"/results?limit=1", nil, "", "owner")
	var results domain.V2ResultPage
	if page.Code != 200 || json.Unmarshal(page.Body.Bytes(), &results) != nil || len(results.Results) != 1 || !results.HasMore || results.NextCursor != "1" || results.LastSequence != "2" || results.Results[0].EventID == "" {
		t.Fatalf("invalid page: %d %s", page.Code, page.Body)
	}
	var linkedPage dto.V2ResultPage
	if err := json.Unmarshal(page.Body.Bytes(), &linkedPage); err != nil || linkedPage.Results[0].Links.Result != base+"/results/bulbasaur" {
		t.Fatalf("missing content link: %s", page.Body)
	}
	frozen := request(t, h, "GET", base+"/results?through=1", nil, "", "owner")
	if frozen.Code != 200 || strings.Contains(frozen.Body.String(), `"nodeId":"language"`) || !strings.Contains(frozen.Body.String(), `"hasMore":false`) {
		t.Fatalf("snapshot changed: %d %s", frozen.Code, frozen.Body)
	}
	detail := request(t, h, "GET", base+"/results/bulbasaur", nil, "", "owner")
	if detail.Code != 200 || !json.Valid(detail.Body.Bytes()) || !strings.Contains(detail.Body.String(), `"data":{"id":9007199254740993}`) || !strings.Contains(detail.Body.String(), `"complete":true`) || !strings.Contains(detail.Body.String(), results.Results[0].EventID) {
		t.Fatalf("invalid complete result: %d %s", detail.Code, detail.Body)
	}
	for _, path := range []string{"/results", "/results/bulbasaur"} {
		if denied := request(t, h, "GET", base+path, nil, "", "other"); denied.Code != 404 {
			t.Fatalf("owner isolation %s: %d", path, denied.Code)
		}
	}
	scalar := request(t, h, "GET", base+"/results/language", nil, "", "owner")
	if scalar.Code != 200 || !json.Valid(scalar.Body.Bytes()) || !strings.Contains(scalar.Body.String(), `"data":"en"`) || !strings.Contains(scalar.Body.String(), `"complete":true`) {
		t.Fatalf("invalid scalar result: %d %s", scalar.Code, scalar.Body)
	}
	if missing := request(t, h, "GET", base+"/results/missing", nil, "", "owner"); missing.Code != 404 {
		t.Fatalf("missing result: %d %s", missing.Code, missing.Body)
	}
	for _, query := range []string{"after=-1", "after=01", "after=3", "after=0&after=1", "limit=101", "limit=0", "through=-1", "through=01", "through=1&through=2", "through=9223372036854775808", "through=3", "after=2&through=1"} {
		if invalid := request(t, h, "GET", base+"/results?"+query, nil, "", "owner"); invalid.Code != 422 {
			t.Fatalf("query %s: %d %s", query, invalid.Code, invalid.Body)
		}
	}
}
