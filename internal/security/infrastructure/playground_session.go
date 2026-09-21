package infrastructure

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/javleds/api-engine/internal/security/domain"
)

var ErrInvalidPlaygroundCredentials = errors.New("invalid playground credentials")

type PlaygroundSessionAuthenticator struct {
	username string
	password string
	key      []byte
	now      func() time.Time
	ttl      time.Duration
}

func NewPlaygroundSessionAuthenticator(username, password string, identityKey []byte, ttl time.Duration) (*PlaygroundSessionAuthenticator, error) {
	if username == "" || password == "" || len(identityKey) == 0 || ttl <= 0 {
		return nil, errors.New("invalid playground session configuration")
	}
	keyMaterial := append([]byte("api-engine/playground-session/v1\x00"), identityKey...)
	digest := sha256.Sum256(keyMaterial)
	return &PlaygroundSessionAuthenticator{username: username, password: password, key: digest[:], now: time.Now, ttl: ttl}, nil
}

func (a *PlaygroundSessionAuthenticator) Login(username, password string) (string, error) {
	if !sameSecret(a.username, username) || !sameSecret(a.password, password) {
		return "", ErrInvalidPlaygroundCredentials
	}
	payload, err := json.Marshal(playgroundSessionClaims{Version: 1, Expires: a.now().Add(a.ttl).Unix()})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return encoded + "." + a.signature(encoded), nil
}

func (a *PlaygroundSessionAuthenticator) Authenticate(_ context.Context, token string) (domain.Principal, error) {
	parts := splitSession(token)
	if len(parts) != 2 || !hmac.Equal([]byte(parts[1]), []byte(a.signature(parts[0]))) {
		return domain.Principal{}, ErrInvalidPlaygroundCredentials
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return domain.Principal{}, ErrInvalidPlaygroundCredentials
	}
	var claims playgroundSessionClaims
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Version != 1 || a.now().Unix() >= claims.Expires {
		return domain.Principal{}, ErrInvalidPlaygroundCredentials
	}
	return domain.Principal{OwnerID: domain.PlaygroundOwnerID, Permissions: []string{"executions:create", "executions:read", "cache:clear"}}, nil
}

func (a *PlaygroundSessionAuthenticator) signature(payload string) string {
	mac := hmac.New(sha256.New, a.key)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func sameSecret(expected, actual string) bool {
	left := sha256.Sum256([]byte(expected))
	right := sha256.Sum256([]byte(actual))
	return subtle.ConstantTimeCompare(left[:], right[:]) == 1
}
func splitSession(token string) []string {
	for index := 0; index < len(token); index++ {
		if token[index] == '.' {
			return []string{token[:index], token[index+1:]}
		}
	}
	return nil
}
