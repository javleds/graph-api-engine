package http_test

import (
	"context"
	"errors"
	"github.com/javleds/api-engine/internal/security/domain"
)

type testAuthenticator struct{}

func (testAuthenticator) Authenticate(_ context.Context, token string) (domain.Principal, error) {
	if token == "" {
		return domain.Principal{}, errors.New("unauthorized")
	}
	return domain.Principal{OwnerID: token, Permissions: []string{"executions:create", "executions:read"}}, nil
}
