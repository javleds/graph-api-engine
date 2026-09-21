package infrastructure

import (
	"context"
	"github.com/javleds/api-engine/internal/security/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestPlaygroundOwnerCannotBeConfiguredAsTokenPrincipal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "principals.json")
	raw := `{"version":1,"principals":[{"ownerId":"system:playground","tokenSHA256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","enabled":true,"permissions":["executions:read"]}]}`
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTokenAuthenticator(path); err == nil {
		t.Fatal("reserved owner accepted")
	}
	p, err := (PlaygroundAuthenticator{}).Authenticate(context.Background(), "ignored")
	if err != nil || p.OwnerID != domain.PlaygroundOwnerID || !p.Can("executions:create") || !p.Can("executions:read") || !p.Can("cache:clear") || p.Can("metrics:read") {
		t.Fatalf("%+v %v", p, err)
	}
}
