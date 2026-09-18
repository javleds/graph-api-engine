package worker

import "encoding/json"

// NodeOutcome owns the immutable JSON produced by one logical invocation.
type NodeOutcome struct {
	Value            json.RawMessage
	OwnDataAvailable bool
	Incomplete       bool
	FailureCount     uint64
}
