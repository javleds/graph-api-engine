package domain

type UpstreamFailure struct {
	Code           string
	HTTPStatus     int
	Attempts       int
	Retryable      bool
	RetryExhausted bool
}

func (e *UpstreamFailure) Error() string { return e.Code }
