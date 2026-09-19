package application

// CreateExecutionCommand contains the normalized, compiled snapshot, never an HTTP request.
type CreateExecutionCommand struct {
	APIVersion     int
	OwnerID        string
	IdempotencyKey string
	Definition     []byte
	WebhookEnabled bool
}
