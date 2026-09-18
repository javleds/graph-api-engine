package dto

type PaginationDTO struct {
	Page     string           `json:"page"`
	PageSize string           `json:"pageSize"`
	Start    *int64           `json:"start"`
	Pages    PageSelectionDTO `json:"pages"`
}
