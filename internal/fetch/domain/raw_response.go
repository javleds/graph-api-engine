package domain

import "time"

type RawResponse struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
	FetchedAt  time.Time
	Attempts   int
}

func (r RawResponse) Clone() RawResponse {
	r.Body = append([]byte(nil), r.Body...)
	headers := make(map[string][]string, len(r.Headers))
	for k, v := range r.Headers {
		headers[k] = append([]string(nil), v...)
	}
	r.Headers = headers
	return r
}
