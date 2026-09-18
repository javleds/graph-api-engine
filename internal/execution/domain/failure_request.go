package domain

type FailureRequest struct {
	Method       string `json:"method"`
	PathTemplate string `json:"pathTemplate"`
}
