package domain

type ProtectedBlob struct {
	Version    int    `json:"version"`
	KeyID      string `json:"keyId"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}
