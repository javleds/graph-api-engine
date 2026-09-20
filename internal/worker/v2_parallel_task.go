package worker

// Parallel tasks own their diagnostics; parents merge them only after durable join.
type v2ParallelTask struct {
	Key        string       `json:"key"`
	Parent     string       `json:"parent,omitempty"`
	Root       int          `json:"root"`
	ItemIndex  *int64       `json:"itemIndex,omitempty"`
	ResultNode string       `json:"resultNode,omitempty"`
	Pending    int          `json:"pending"`
	Checkpoint v2Checkpoint `json:"checkpoint"`
}
