package domain

// AttemptBudgetExceeded rejects sponsorship by one execution, not a shared flight.
type AttemptBudgetExceeded struct{}

func (AttemptBudgetExceeded) Error() string { return "EXECUTION_BUDGET_EXCEEDED" }
