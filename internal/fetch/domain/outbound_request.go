package domain

type OutboundRequest struct {
	Method  string
	URL     string
	Headers map[string][]string
	Body    []byte
}
