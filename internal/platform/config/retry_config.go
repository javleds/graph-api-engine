package config

type RetryConfig struct {
	MaxAttempts int      `json:"maxAttempts"`
	BaseDelay   Duration `json:"baseDelay"`
	MaxBackoff  Duration `json:"maxBackoff"`
}
