package domain

type TemplateReference struct {
	Start   int
	End     int
	Context string
	Path    []string
}
