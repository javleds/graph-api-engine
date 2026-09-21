package http_test

import (
	"context"
	"net/http"
	"testing"

	api "github.com/javleds/api-engine/internal/api/http"
	definitionapp "github.com/javleds/api-engine/internal/definition/application"
	"github.com/javleds/api-engine/internal/platform/clock"
	securitydomain "github.com/javleds/api-engine/internal/security/domain"
)

type cacheClearAuthenticator struct{ permissions []string }

func (a cacheClearAuthenticator) Authenticate(_ context.Context, token string) (securitydomain.Principal, error) {
	return securitydomain.Principal{OwnerID: token, Permissions: a.permissions}, nil
}

func TestClearCacheRequiresDedicatedPermissionAndServesBothVersions(t *testing.T) {
	var calls int
	server := &api.Server{
		Auth:           cacheClearAuthenticator{permissions: []string{"cache:clear"}},
		PlaygroundAuth: cacheClearAuthenticator{permissions: []string{"cache:clear"}},
		Clock:          clock.RealClock{},
		Limits:         definitionapp.DefaultValidationLimits(),
		ClearCache:     func(context.Context) (int64, error) { calls++; return 3, nil },
	}
	h := server.Handler()
	for _, path := range []string{"/v1/cache/clear", "/v2/cache/clear", "/playground/api/cache/clear"} {
		got := request(t, h, http.MethodPost, path, nil, "", "operator")
		if got.Code != http.StatusOK || got.Body.String() != `{"deletedEntries":3}` {
			t.Fatalf("%s: %d %s", path, got.Code, got.Body)
		}
	}
	if calls != 3 {
		t.Fatalf("clear calls = %d, want 3", calls)
	}

	denied := &api.Server{Auth: cacheClearAuthenticator{}, Clock: clock.RealClock{}, Limits: definitionapp.DefaultValidationLimits()}
	if got := request(t, denied.Handler(), http.MethodPost, "/v1/cache/clear", nil, "", "operator"); got.Code != http.StatusForbidden {
		t.Fatalf("permission check: %d %s", got.Code, got.Body)
	}
}
