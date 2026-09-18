package domain

type Pagination struct {
	Page     string
	PageSize string
	Start    int64
	Pages    PageSelection
}
