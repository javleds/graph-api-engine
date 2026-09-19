package domain

import "encoding/json"

type V2Input struct {
	Key     string
	Payload json.RawMessage
}
