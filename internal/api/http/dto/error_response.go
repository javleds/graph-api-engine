package dto

type ErrorResponse struct {
	Error     PublicError `json:"error"`
	RequestID string      `json:"requestId"`
	ID        string      `json:"id,omitempty"`
}
