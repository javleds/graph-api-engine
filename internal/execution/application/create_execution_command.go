package application

// CreateExecutionCommand contains the normalized, compiled snapshot, never an HTTP request.
type CreateExecutionCommand struct {
	APIVersion     int
	OwnerID        string
	IdempotencyKey string
	Definition     []byte
	// FingerprintDefinition preserves canonical request identity across snapshot upgrades.
	// When omitted, Definition is used. It is never persisted as the execution snapshot.
	FingerprintDefinition []byte
	WebhookEnabled        bool
}
