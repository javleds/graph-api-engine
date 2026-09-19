package domain

import "encoding/json"

type V2Node struct {
	NodeID   string          `json:"nodeId"`
	ParentID string          `json:"parentId,omitempty"`
	Key      string          `json:"key,omitempty"`
	Index    *int64          `json:"index,omitempty"`
	Path     string          `json:"path,omitempty"`
	Kind     string          `json:"kind"`
	Value    json.RawMessage `json:"value,omitempty"`
	Status   string          `json:"status"`
	Cursor   string          `json:"cursor"`
}
