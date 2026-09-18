package http

import "net/http"

type capturedRequest struct {
	headers http.Header
	body    string
	method  string
}
