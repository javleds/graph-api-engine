package http_test

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	api "github.com/javleds/api-engine/internal/api/http"
	"github.com/javleds/api-engine/internal/api/http/dto"
)

func TestCancelRoutesAreDurableIdempotentAndOwnerScoped(t *testing.T) {
	for _, base := range []string{"/v1/executions", "/v2/executions", "/playground/api/executions", "/playground/api/v2/executions"} {
		t.Run(base, func(t *testing.T) {
			h, _, dispatcher := newTestAPI(t)
			created := request(t, h, "POST", base, v2Request, "", "owner")
			if created.Code != 202 {
				t.Fatalf("create %d %s", created.Code, created.Body)
			}
			var accepted dto.CreateExecutionResponse
			json.Unmarshal(created.Body.Bytes(), &accepted)
			path := base + "/" + string(accepted.ID) + "/cancel"
			if !strings.HasPrefix(base, "/playground") {
				if denied := request(t, h, "POST", path, nil, "", "other"); denied.Code != 404 {
					t.Fatalf("owner isolation %d", denied.Code)
				}
				wrong := "/v2/executions/"
				if strings.Contains(base, "v2") {
					wrong = "/v1/executions/"
				}
				if denied := request(t, h, "POST", wrong+string(accepted.ID)+"/cancel", nil, "", "owner"); denied.Code != 404 {
					t.Fatalf("version isolation %d", denied.Code)
				}
			} else {
				r := httptest.NewRequest("POST", path, nil)
				r.Header.Set("Sec-Fetch-Site", "cross-site")
				denied := httptest.NewRecorder()
				h.ServeHTTP(denied, r)
				if denied.Code != 403 {
					t.Fatalf("crossorigin %d", denied.Code)
				}
			}
			for range 2 {
				response := request(t, h, "POST", path, nil, "", "owner")
				var cancelled dto.CancelExecutionResponse
				json.Unmarshal(response.Body.Bytes(), &cancelled)
				if response.Code != 200 || cancelled.Error == nil || cancelled.Error.Code != "EXECUTION_CANCELLED" || cancelled.Status != "failed" {
					t.Fatalf("cancel %d %s", response.Code, response.Body)
				}
			}
			if len(dispatcher.cancelled) == 0 {
				t.Fatal("runtime never signalled")
			}
		})
	}
}

func TestCancelRequiresReadAndCreatePermissions(t *testing.T) {
	for _, permission := range []string{"executions:read", "executions:create"} {
		h := (&api.Server{Auth: cancelPermissionAuthenticator{permission: permission}}).Handler()
		for _, version := range []string{"v1", "v2"} {
			response := request(t, h, "POST", "/"+version+"/executions/id/cancel", nil, "", "owner")
			if response.Code != 403 {
				t.Fatalf("permission %s %d", permission, response.Code)
			}
		}
	}
}
