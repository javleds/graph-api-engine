package config

type EncryptionConfig struct {
	ActiveKeyID string            `json:"activeKeyId" yaml:"activeKeyId"`
	KeyFiles    map[string]string `json:"keyFiles" yaml:"keyFiles"`
}
