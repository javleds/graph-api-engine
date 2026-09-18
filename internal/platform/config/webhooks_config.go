package config

type WebhooksConfig struct {
	MaxConcurrent        int      `json:"maxConcurrent"`
	MaxPendingDeliveries int      `json:"maxPendingDeliveries"`
	PollInterval         Duration `json:"pollInterval"`
	ClaimLease           Duration `json:"claimLease"`
	AttemptTimeout       Duration `json:"attemptTimeout"`
	MaxResponseBytes     int      `json:"maxResponseBytes"`
	MaxPayloadBytes      int      `json:"maxPayloadBytes"`
	MaxAttempts          int      `json:"maxAttempts"`
	BaseDelay            Duration `json:"baseDelay"`
	MaxBackoff           Duration `json:"maxBackoff"`
	DeliveryMaxAge       Duration `json:"deliveryMaxAge"`
}
