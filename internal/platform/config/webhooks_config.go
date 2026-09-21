package config

type WebhooksConfig struct {
	MaxConcurrent        int      `json:"maxConcurrent" yaml:"maxConcurrent" validate:"gt=0"`
	MaxPendingDeliveries int      `json:"maxPendingDeliveries" yaml:"maxPendingDeliveries" validate:"gt=0"`
	PollInterval         Duration `json:"pollInterval" yaml:"pollInterval" validate:"gt=0"`
	ClaimLease           Duration `json:"claimLease" yaml:"claimLease" validate:"gt=0"`
	AttemptTimeout       Duration `json:"attemptTimeout" yaml:"attemptTimeout" validate:"gt=0"`
	MaxResponseBytes     int      `json:"maxResponseBytes" yaml:"maxResponseBytes" validate:"gt=0"`
	MaxPayloadBytes      int      `json:"maxPayloadBytes" yaml:"maxPayloadBytes" validate:"gt=0"`
	MaxAttempts          int      `json:"maxAttempts" yaml:"maxAttempts" validate:"gt=0"`
	BaseDelay            Duration `json:"baseDelay" yaml:"baseDelay" validate:"gt=0"`
	MaxBackoff           Duration `json:"maxBackoff" yaml:"maxBackoff" validate:"gt=0"`
	DeliveryMaxAge       Duration `json:"deliveryMaxAge" yaml:"deliveryMaxAge" validate:"gt=0"`
}
