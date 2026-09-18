package integration

import "net/http"

type capturedWebhook struct {
	Header http.Header
	Body   []byte
}
