package domain

import "encoding/json"

type V2Task struct {
	Key       string
	ParentKey string
	Priority  int64
	Revision  int64
	Done      bool
	Payload   json.RawMessage
}
