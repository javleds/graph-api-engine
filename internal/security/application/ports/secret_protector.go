package ports

import (
	"context"
	"github.com/javleds/api-engine/internal/security/domain"
)

type SecretProtector interface {
	Seal(context.Context, string, []byte) (domain.ProtectedBlob, error)
	Open(context.Context, string, domain.ProtectedBlob) ([]byte, error)
}
