package worker

// Pointer addresses a value within a saved selected page; Normalize exposes the
// same frozen scalar wrapper to child bindings as the v1 assembler.
type v2InputRef struct {
	Key       string   `json:"key"`
	Pointer   []string `json:"pointer,omitempty"`
	Normalize bool     `json:"normalize,omitempty"`
}
