package dto

type RequestDTO struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   map[string]any    `json:"query"`
	Headers map[string]string `json:"headers"`
}
