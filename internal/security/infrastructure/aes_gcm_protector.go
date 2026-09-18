package infrastructure

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/javleds/api-engine/internal/security/domain"
)

type AESGCMProtector struct {
	activeKeyID string
	keys        map[string]cipher.AEAD
}

func NewAESGCMProtector(activeKeyID string, keys map[string][]byte) (*AESGCMProtector, error) {
	p := &AESGCMProtector{activeKeyID: activeKeyID, keys: make(map[string]cipher.AEAD)}
	for id, key := range keys {
		if id == "" || len(key) != 32 {
			return nil, errors.New("encryption keys require an identifier and exactly 32 bytes")
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		p.keys[id] = aead
	}
	if _, ok := p.keys[activeKeyID]; !ok {
		return nil, errors.New("active encryption key is unavailable")
	}
	return p, nil
}

func (p *AESGCMProtector) Seal(ctx context.Context, purpose string, plaintext []byte) (domain.ProtectedBlob, error) {
	if err := ctx.Err(); err != nil {
		return domain.ProtectedBlob{}, err
	}
	aead := p.keys[p.activeKeyID]
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return domain.ProtectedBlob{}, fmt.Errorf("generate encryption nonce: %w", err)
	}
	aad, _ := json.Marshal([]string{"1", p.activeKeyID, purpose})
	return domain.ProtectedBlob{Version: 1, KeyID: p.activeKeyID, Nonce: nonce, Ciphertext: aead.Seal(nil, nonce, plaintext, aad)}, nil
}

func (p *AESGCMProtector) Open(ctx context.Context, purpose string, blob domain.ProtectedBlob) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	aead, ok := p.keys[blob.KeyID]
	if blob.Version != 1 || !ok {
		return nil, errors.New("unsupported protected blob or unavailable key")
	}
	if len(blob.Nonce) != aead.NonceSize() {
		return nil, errors.New("invalid encryption nonce")
	}
	aad, _ := json.Marshal([]string{"1", blob.KeyID, purpose})
	plaintext, err := aead.Open(nil, blob.Nonce, blob.Ciphertext, aad)
	if err != nil {
		return nil, errors.New("protected blob authentication failed")
	}
	return plaintext, nil
}
