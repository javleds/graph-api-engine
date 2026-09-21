package infrastructure

import (
	"context"
	"testing"
	"time"
)

func TestPlaygroundSessionAuthenticatesAndExpires(t *testing.T) {
	auth, err := NewPlaygroundSessionAuthenticator("admin", "password", []byte("01234567890123456789012345678901"), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	auth.now = func() time.Time { return now }
	token, err := auth.Login("admin", "password")
	if err != nil {
		t.Fatal(err)
	}
	if principal, err := auth.Authenticate(context.Background(), token); err != nil || principal.OwnerID == "" || !principal.Can("cache:clear") {
		t.Fatalf("principal=%#v err=%v", principal, err)
	}
	now = now.Add(time.Hour)
	if _, err := auth.Authenticate(context.Background(), token); err == nil {
		t.Fatal("expired session accepted")
	}
	if _, err := auth.Login("admin", "wrong"); err == nil {
		t.Fatal("wrong password accepted")
	}
}
