package domain

type V2TreePage struct {
	Nodes        []V2Node `json:"nodes"`
	NextCursor   string   `json:"nextCursor"`
	Terminal     bool     `json:"terminal"`
	LastSequence string   `json:"lastSequence"`
	HasMore      bool     `json:"hasMore"`
}
