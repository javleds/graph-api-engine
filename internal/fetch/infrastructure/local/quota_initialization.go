package local

// Each load has its own outcome so old waiters cannot observe a later retry.
type quotaInitialization struct {
	ready chan struct{}
	err   error
}
