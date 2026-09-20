package dto

type V2ResultPage struct {
	Results      []V2ResultResponse `json:"results"`
	NextCursor   string             `json:"nextCursor"`
	LastSequence string             `json:"lastSequence"`
	HasMore      bool               `json:"hasMore"`
	Terminal     bool               `json:"terminal"`
}
