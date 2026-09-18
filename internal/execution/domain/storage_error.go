package domain

type StorageError string

func (e StorageError) Error() string { return string(e) }

const (
	ErrEngineBusy          StorageError = "ENGINE_BUSY"
	ErrNotFound            StorageError = "EXECUTION_NOT_FOUND"
	ErrIdempotencyConflict StorageError = "IDEMPOTENCY_CONFLICT"
	ErrNotReady            StorageError = "RESULT_NOT_READY"
	ErrExecutionFailed     StorageError = "EXECUTION_FAILED"
	ErrResultExpired       StorageError = "RESULT_EXPIRED"
	ErrVersionConflict     StorageError = "EXECUTION_VERSION_CONFLICT"
)
