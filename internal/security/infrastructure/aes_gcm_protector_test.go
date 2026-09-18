package infrastructure

import (
	"bytes"
	"context"
	"testing"
)

func TestEncryptionAuthenticationRotation(t *testing.T) {
	ctx := context.Background()
	oldKey := bytes.Repeat([]byte{1}, 32)
	newKey := bytes.Repeat([]byte{2}, 32)
	old, err := NewAESGCMProtector("old", map[string][]byte{"old": oldKey})
	if err != nil {
		t.Fatal(err)
	}
	blob, err := old.Seal(ctx, "owner/a", []byte("sensitive bearer secret"))
	if err != nil {
		t.Fatal(err)
	}
	second, _ := old.Seal(ctx, "owner/a", []byte("sensitive bearer secret"))
	if bytes.Equal(blob.Nonce, second.Nonce) {
		t.Fatal("reused nonce")
	}
	rotated, err := NewAESGCMProtector("new", map[string][]byte{"old": oldKey, "new": newKey})
	if err != nil {
		t.Fatal(err)
	}
	plain, err := rotated.Open(ctx, "owner/a", blob)
	if err != nil || string(plain) != "sensitive bearer secret" {
		t.Fatalf("rotation: %s %v", plain, err)
	}
	if _, err := rotated.Open(ctx, "owner/b", blob); err == nil {
		t.Fatal("owner substitution accepted")
	}
	blob.Ciphertext[0] ^= 1
	if _, err := rotated.Open(ctx, "owner/a", blob); err == nil {
		t.Fatal("tampering accepted")
	}
	if _, err := NewAESGCMProtector("bad", map[string][]byte{"bad": {1}}); err == nil {
		t.Fatal("short key accepted")
	}
}
