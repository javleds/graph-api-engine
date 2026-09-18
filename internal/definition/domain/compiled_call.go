package domain

type CompiledCall struct {
	Name       string
	Type       string
	Select     []string
	Request    RequestTemplate
	Pagination *Pagination
	Children   []CompiledCall
	Limits     *RatePolicy
	Cache      CachePolicy
	Pointer    string
	RootIndex  int
}
