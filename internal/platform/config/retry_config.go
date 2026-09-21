package config

type RetryConfig struct {
	MaxAttempts int      `json:"maxAttempts" yaml:"maxAttempts" validate:"gt=0"`
	BaseDelay   Duration `json:"baseDelay" yaml:"baseDelay" validate:"gt=0"`
	MaxBackoff  Duration `json:"maxBackoff" yaml:"maxBackoff" validate:"gt=0"`
}
