package worker

// v2Frame is a durable continuation. Values stay in encrypted input records;
// continuations retain only references and cursors, never a growing result tree.
type v2Frame struct {
	Kind       string                `json:"kind"`
	Phase      string                `json:"phase,omitempty"`
	Call       []int                 `json:"call,omitempty"`
	Ref        v2InputRef            `json:"ref"`
	Ancestors  map[string]v2InputRef `json:"ancestors,omitempty"`
	NodeID     string                `json:"nodeId"`
	ParentID   string                `json:"parentId,omitempty"`
	Key        string                `json:"key,omitempty"`
	Index      *int64                `json:"index,omitempty"`
	Path       string                `json:"path"`
	Cursor     int                   `json:"cursor,omitempty"`
	Page       int64                 `json:"page,omitempty"`
	Accepted   int64                 `json:"accepted,omitempty"`
	Expected   int64                 `json:"expected,omitempty"`
	Offset     int64                 `json:"offset,omitempty"`
	Previous   string                `json:"previous,omitempty"`
	Stop       bool                  `json:"stop,omitempty"`
	Available  bool                  `json:"available,omitempty"`
	ErrorStart uint64                `json:"errorStart,omitempty"`
}
