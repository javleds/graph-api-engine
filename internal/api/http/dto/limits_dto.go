package dto

type LimitsDTO struct {
	Requests    int64  `json:"requests"`
	Per         string `json:"per"`
	Concurrency int    `json:"concurrency"`
}
