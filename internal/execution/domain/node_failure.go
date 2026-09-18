package domain

type NodeFailure struct {
	Code               string          `json:"code"`
	Message            string          `json:"message"`
	Scope              string          `json:"scope"`
	NodePath           *string         `json:"nodePath"`
	DefinitionPath     *string         `json:"definitionPath"`
	Stage              string          `json:"stage"`
	UpstreamStatus     *int            `json:"upstreamStatus"`
	Attempts           int             `json:"attempts"`
	Retryable          bool            `json:"retryable"`
	RetryExhausted     bool            `json:"retryExhausted"`
	DescendantsSkipped bool            `json:"descendantsSkipped"`
	Page               *int            `json:"page,omitempty"`
	Request            *FailureRequest `json:"request,omitempty"`
}
