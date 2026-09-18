package domain

import "testing"

func validCompletion() ExecutionCompletion {
	report := EmptyReport()
	report.Roots = []RootOutcome{{Name: "items", Status: Completed, DataAvailable: true}}
	return ExecutionCompletion{ID: "job", Status: Completed, Report: report, Data: []byte(`{"items":[]}`)}
}

func TestCompletionInvariants(t *testing.T) {
	mutations := map[string]func(*ExecutionCompletion){
		"nonterminal":            func(c *ExecutionCompletion) { c.Status = Running },
		"null errors":            func(c *ExecutionCompletion) { c.Report.Errors = nil },
		"null roots":             func(c *ExecutionCompletion) { c.Report.Roots = nil },
		"returned mismatch":      func(c *ExecutionCompletion) { c.Report.ErrorSummary.Returned = 1 },
		"total below returned":   func(c *ExecutionCompletion) { c.Report.Errors = []NodeFailure{{}}; c.Report.ErrorSummary.Returned = 1 },
		"truncated mismatch":     func(c *ExecutionCompletion) { c.Report.ErrorSummary.Truncated = true },
		"completed errors":       func(c *ExecutionCompletion) { c.Report.ErrorSummary = ErrorSummary{Total: 1, Truncated: true} },
		"partial without errors": func(c *ExecutionCompletion) { c.Status = CompletedWithErrors },
		"incomplete root":        func(c *ExecutionCompletion) { c.Report.Roots[0].Status = CompletedWithErrors },
		"no available root": func(c *ExecutionCompletion) {
			c.Report.Roots[0].Status = Failed
			c.Report.Roots[0].DataAvailable = false
		},
		"inconsistent root": func(c *ExecutionCompletion) { c.Report.Roots[0].DataAvailable = false },
		"failed with data": func(c *ExecutionCompletion) {
			c.Status = Failed
			c.Error = &ExecutionFailure{Code: "FAILED", Message: "Fallo"}
		},
		"failed without error": func(c *ExecutionCompletion) { c.Status = Failed; c.Data = nil },
		"success with error":   func(c *ExecutionCompletion) { c.Error = &ExecutionFailure{Code: "FAILED", Message: "Fallo"} },
		"missing root":         func(c *ExecutionCompletion) { c.Data = []byte(`{}`) },
		"different root":       func(c *ExecutionCompletion) { c.Data = []byte(`{"other":[]}`) },
		"extra root":           func(c *ExecutionCompletion) { c.Data = []byte(`{"items":[],"other":[]}`) },
		"array result":         func(c *ExecutionCompletion) { c.Data = []byte(`[]`) },
		"null result":          func(c *ExecutionCompletion) { c.Data = []byte(`null`) },
		"invalid JSON":         func(c *ExecutionCompletion) { c.Data = []byte(`{`) },
		"duplicate roots":      func(c *ExecutionCompletion) { c.Report.Roots = append(c.Report.Roots, c.Report.Roots[0]) },
		"empty root name":      func(c *ExecutionCompletion) { c.Report.Roots[0].Name = "" },
		"nonterminal root":     func(c *ExecutionCompletion) { c.Report.Roots[0].Status = Running },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			c := validCompletion()
			mutate(&c)
			if err := ValidateCompletion(c); err == nil {
				t.Fatal("invalid completion accepted")
			}
		})
	}
}

func TestValidTerminalOutcomes(t *testing.T) {
	completed := validCompletion()
	if err := ValidateCompletion(completed); err != nil {
		t.Fatal(err)
	}
	partial := validCompletion()
	partial.Status = CompletedWithErrors
	partial.Report.Roots[0].Status = CompletedWithErrors
	partial.Report.ErrorSummary = ErrorSummary{Total: 2, Truncated: true}
	partial.Report.Roots = append(partial.Report.Roots, RootOutcome{Name: "missing", Status: Failed})
	partial.Data = []byte(`{"items":[],"missing":null}`)
	if err := ValidateCompletion(partial); err != nil {
		t.Fatal(err)
	}
	interrupted := ExecutionCompletion{Status: Failed, Error: &ExecutionFailure{Code: "PROCESS_INTERRUPTED", Message: "Proceso interrumpido."}, Report: EmptyReport()}
	if err := ValidateCompletion(interrupted); err != nil {
		t.Fatal(err)
	}
	technical := completed
	technical.Status = Failed
	technical.Error = interrupted.Error
	technical.Data = nil
	if err := ValidateCompletion(technical); err != nil {
		t.Fatal(err)
	}
}
