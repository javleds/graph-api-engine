package dto

type ExecutionLinks struct {
	Results string `json:"results,omitempty"`
	Chunks  string `json:"chunks,omitempty"`
	Tree    string `json:"tree,omitempty"`
	Export  string `json:"export,omitempty"`
	Self    string `json:"self"`
	Result  string `json:"result,omitempty"`
}
