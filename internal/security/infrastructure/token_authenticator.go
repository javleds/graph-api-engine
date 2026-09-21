package infrastructure

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/javleds/api-engine/internal/security/domain"
	"io"
	"os"
	"strings"
)

type TokenAuthenticator struct {
	principals  []PrincipalRecord
	Development bool
}

func LoadTokenAuthenticator(path string) (*TokenAuthenticator, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	dec := json.NewDecoder(io.LimitReader(file, 1<<20))
	dec.DisallowUnknownFields()
	var input PrincipalsFile
	if err := dec.Decode(&input); err != nil {
		return nil, fmt.Errorf("invalid principals file")
	}
	if dec.Decode(new(any)) != io.EOF || input.Version != 1 || len(input.Principals) == 0 {
		return nil, fmt.Errorf("invalid principals file")
	}
	seen := map[string]bool{}
	for _, p := range input.Principals {
		hash, err := hex.DecodeString(p.TokenSHA256)
		if err != nil || len(hash) != 32 || p.OwnerID == "" || p.OwnerID == domain.PlaygroundOwnerID || seen[strings.ToLower(p.TokenSHA256)] {
			return nil, fmt.Errorf("invalid principal")
		}
		seen[strings.ToLower(p.TokenSHA256)] = true
		for _, perm := range p.Permissions {
			if perm != "executions:create" && perm != "executions:read" && perm != "metrics:read" && perm != "cache:clear" {
				return nil, fmt.Errorf("unknown permission")
			}
		}
	}
	return &TokenAuthenticator{principals: input.Principals}, nil
}
func (a *TokenAuthenticator) Authenticate(_ context.Context, token string) (domain.Principal, error) {
	if a.Development {
		return domain.Principal{OwnerID: "development", Permissions: []string{"executions:create", "executions:read", "metrics:read", "cache:clear"}}, nil
	}
	hash := sha256.Sum256([]byte(token))
	var result domain.Principal
	for _, p := range a.principals {
		expected, _ := hex.DecodeString(p.TokenSHA256)
		if subtle.ConstantTimeCompare(hash[:], expected) == 1 && p.Enabled {
			result = domain.Principal{OwnerID: p.OwnerID, Permissions: append([]string(nil), p.Permissions...)}
		}
	}
	if token == "" || result.OwnerID == "" {
		return domain.Principal{}, fmt.Errorf("UNAUTHENTICATED")
	}
	return result, nil
}
