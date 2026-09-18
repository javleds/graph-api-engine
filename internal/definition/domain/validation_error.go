package domain

// ValidationError contains safe, stable diagnostics; never include input secrets.
type ValidationError struct {
	Code    string
	Path    string
	Message string
}

func (e *ValidationError) Error() string { return e.Code + " at " + e.Path + ": " + e.Message }
func Invalid(path, message string) error {
	return &ValidationError{Code: "INVALID_DEFINITION", Path: path, Message: message}
}
