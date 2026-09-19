package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	api "github.com/javleds/api-engine/internal/api/http"
	"github.com/javleds/api-engine/internal/api/http/dto"
	"github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/platform/clock"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

var v2Request = []byte(`{"definition":{"version":1,"api":{"baseUrl":"https://example.com","limits":{"requests":100,"per":"1m"}},"calls":{"name":"items","type":"collection","request":{"method":"GET","path":"/items"}}}}`)

func TestV2VersionIsolationAndValidation(t *testing.T) {
	h, _, _ := newTestAPI(t)
	ids := map[string]string{}
	for _, version := range []string{"v1", "v2"} {
		response := request(t, h, "POST", "/"+version+"/executions", v2Request, "shared", "owner")
		if response.Code != 202 {
			t.Fatalf("%s %d %s", version, response.Code, response.Body)
		}
		var body dto.CreateExecutionResponse
		json.Unmarshal(response.Body.Bytes(), &body)
		ids[version] = string(body.ID)
	}
	if ids["v1"] == ids["v2"] {
		t.Fatal("idempotency namespaces overlap")
	}
	for _, path := range []string{"/v1/executions/" + ids["v2"], "/v1/executions/" + ids["v2"] + "/result", "/v2/executions/" + ids["v1"], "/v2/executions/" + ids["v1"] + "/chunks"} {
		if response := request(t, h, "GET", path, nil, "", "owner"); response.Code != 404 {
			t.Fatalf("%s: %d %s", path, response.Code, response.Body)
		}
	}
	invalid := append(append([]byte{}, v2Request[:len(v2Request)-1]...), []byte(`,"webhook":{"url":"https://example.com/hook","payload":"result"}}`)...)
	for _, path := range []string{"/v2/executions", "/playground/api/v2/validate"} {
		if response := request(t, h, "POST", path, invalid, "", "owner"); response.Code != 422 || !strings.Contains(response.Body.String(), "/webhook/payload") {
			t.Fatalf("%s: %d %s", path, response.Code, response.Body)
		}
	}
	cross := httptest.NewRequest("POST", "http://localhost/playground/api/v2/validate", bytes.NewReader(v2Request))
	cross.Header.Set("Content-Type", "application/json")
	cross.Header.Set("Sec-Fetch-Site", "cross-site")
	denied := httptest.NewRecorder()
	h.ServeHTTP(denied, cross)
	if denied.Code != 403 {
		t.Fatal(denied.Code)
	}
}

func TestV2ProgressiveReadsAndExport(t *testing.T) {
	h, store, _ := newTestAPI(t)
	accepted := request(t, h, "POST", "/v2/executions", v2Request, "", "owner")
	if accepted.Code != 202 {
		t.Fatalf("%d %s", accepted.Code, accepted.Body)
	}
	var body dto.CreateExecutionResponse
	json.Unmarshal(accepted.Body.Bytes(), &body)
	state, err := store.ReadV2State(context.Background(), body.ID)
	if err != nil {
		t.Fatal(err)
	}
	err = store.CommitV2Task(context.Background(), domain.V2Commit{ID: body.ID, OwnerID: "owner", Generation: state.Generation, Chunks: []domain.V2Chunk{{Type: "data", Operation: "object", NodeID: "root", Key: "items"}, {Type: "data", Operation: "value", NodeID: "value", ParentID: "root", Key: "id", Value: json.RawMessage(`9007199254740993`)}}})
	if err != nil {
		t.Fatal(err)
	}
	base := "/v2/executions/" + string(body.ID)
	for _, suffix := range []string{"", "/chunks", "/tree", "/export"} {
		if response := request(t, h, "GET", base+suffix, nil, "", "other"); response.Code != 404 {
			t.Fatalf("%s %d", suffix, response.Code)
		}
	}
	for _, query := range []string{"after=-1", "after=01", "after=3", "after=0&after=1", "limit=101", "limit=0"} {
		if response := request(t, h, "GET", base+"/chunks?"+query, nil, "", "owner"); response.Code != 422 {
			t.Fatalf("%s %d %s", query, response.Code, response.Body)
		}
	}
	page := request(t, h, "GET", base+"/chunks?limit=1", nil, "", "owner")
	if page.Code != 200 || !strings.Contains(page.Body.String(), `"hasMore":true`) || !strings.Contains(page.Body.String(), `"nextCursor":"1"`) {
		t.Fatal(page.Body)
	}
	tree := request(t, h, "GET", base+"/tree?parentId=root", nil, "", "owner")
	if tree.Code != 200 || !strings.Contains(tree.Body.String(), `9007199254740993`) {
		t.Fatal(tree.Body)
	}
	export := request(t, h, "GET", base+"/export", nil, "", "owner")
	lines := strings.Split(strings.TrimSpace(export.Body.String()), "\n")
	if export.Code != 200 || len(lines) != 4 || !strings.Contains(lines[0], `"terminal":false`) || !strings.Contains(lines[3], `"type":"export_end"`) {
		t.Fatalf("%d %s", export.Code, export.Body)
	}
	for _, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatal(line)
		}
	}
	if !strings.Contains(lines[2], `9007199254740993`) {
		t.Fatal("number precision lost")
	}
}

func TestV2ExportCutoffRemainsStableWhilePublishing(t *testing.T) {
	h, store, _ := newTestAPI(t)
	accepted := request(t, h, "POST", "/v2/executions", v2Request, "", "owner")
	var body dto.CreateExecutionResponse
	json.Unmarshal(accepted.Body.Bytes(), &body)
	state, err := store.ReadV2State(context.Background(), body.ID)
	if err != nil {
		t.Fatal(err)
	}
	chunks := make([]domain.V2Chunk, 150)
	for i := range chunks {
		chunks[i] = domain.V2Chunk{Type: "data", Operation: "value", NodeID: strconv.Itoa(i), Value: json.RawMessage(`null`)}
	}
	commit := func(chunks []domain.V2Chunk) {
		t.Helper()
		if err := store.CommitV2Task(context.Background(), domain.V2Commit{ID: body.ID, OwnerID: "owner", Generation: state.Generation, Chunks: chunks}); err != nil {
			t.Fatal(err)
		}
	}
	commit(chunks)
	wrapped := &testV2ExportStore{V2Store: store, afterFirstRead: func() {
		commit([]domain.V2Chunk{{Type: "data", Operation: "value", NodeID: "late", Value: json.RawMessage(`true`)}})
	}}
	server := &api.Server{V2Store: wrapped, Reader: store, Auth: testAuthenticator{}, Clock: clock.RealClock{}}
	response := request(t, server.Handler(), "GET", "/v2/executions/"+string(body.ID)+"/export", nil, "", "owner")
	lines := strings.Split(strings.TrimSpace(response.Body.String()), "\n")
	if response.Code != 200 || len(lines) != 152 || strings.Contains(response.Body.String(), `"nodeId":"late"`) {
		t.Fatalf("%d lines=%d %s", response.Code, len(lines), response.Body)
	}
	if !strings.Contains(lines[151], `"lastSequence":"150"`) {
		t.Fatal(lines[151])
	}
}
