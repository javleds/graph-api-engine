package infrastructure

import (
	"context"
	"github.com/javleds/api-engine/internal/security/domain"
)

// PlaygroundAuthenticator is mounted only on the open playground execution routes.
type PlaygroundAuthenticator struct{}

func (PlaygroundAuthenticator) Authenticate(context.Context, string) (domain.Principal, error) {
	return domain.Principal{OwnerID: domain.PlaygroundOwnerID, Permissions: []string{"executions:create", "executions:read"}}, nil
}
