package http_test

import (
	"context"
	"github.com/javleds/api-engine/internal/security/domain"
)

type cancelPermissionAuthenticator struct{ permission string }

func (a cancelPermissionAuthenticator) Authenticate(context.Context, string) (domain.Principal, error) {
	return domain.Principal{OwnerID: "owner", Permissions: []string{a.permission}}, nil
}
