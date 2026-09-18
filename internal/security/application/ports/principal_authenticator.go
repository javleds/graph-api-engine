package ports

import (
	"context"
	"github.com/javleds/api-engine/internal/security/domain"
)

type PrincipalAuthenticator interface {
	Authenticate(context.Context, string) (domain.Principal, error)
}
