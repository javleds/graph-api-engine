package config

type EncryptionConfig struct {
	ActiveKeyID string            `json:"activeKeyId"`
	KeyFiles    map[string]string `json:"keyFiles"`
}
