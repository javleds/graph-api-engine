package domain

type ExecutionErrorReport struct {
	Errors       []NodeFailure `json:"errors"`
	ErrorSummary ErrorSummary  `json:"errorSummary"`
	Roots        []RootOutcome `json:"roots"`
}

func EmptyReport() ExecutionErrorReport {
	return ExecutionErrorReport{Errors: []NodeFailure{}, Roots: []RootOutcome{}}
}
