package dto

type CallDTO struct {
	Name       string          `json:"name"`
	Type       string          `json:"type"`
	Select     *string         `json:"select"`
	Request    RequestDTO      `json:"request"`
	Pagination *PaginationDTO  `json:"pagination"`
	Children   []CallDTO       `json:"children"`
	Limits     *LimitsDTO      `json:"limits"`
	Cache      *CachePolicyDTO `json:"cache"`
}
