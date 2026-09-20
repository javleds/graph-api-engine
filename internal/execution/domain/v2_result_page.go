package domain

type V2ResultPage struct {
	Results      []V2Result `json:"results"`
	NextCursor   string     `json:"nextCursor"`
	LastSequence string     `json:"lastSequence"`
	HasMore      bool       `json:"hasMore"`
	Terminal     bool       `json:"terminal"`
}
