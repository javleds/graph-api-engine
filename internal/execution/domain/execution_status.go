package domain

type ExecutionStatus string

const (
	Pending             ExecutionStatus = "pending"
	Running             ExecutionStatus = "running"
	Completed           ExecutionStatus = "completed"
	CompletedWithErrors ExecutionStatus = "completed_with_errors"
	Failed              ExecutionStatus = "failed"
)

func (s ExecutionStatus) Terminal() bool {
	return s == Completed || s == CompletedWithErrors || s == Failed
}
