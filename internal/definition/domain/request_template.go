package domain

type RequestTemplate struct {
	Method  string
	Path    Template
	Query   map[string]any
	Headers map[string]string
}
