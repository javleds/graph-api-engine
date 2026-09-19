package domain

type V2Commit struct {
	Progress     *ExecutionProgress
	ID           ExecutionID
	OwnerID      string
	Generation   int64
	Task         V2Task
	NewTasks     []V2Task
	Chunks       []V2Chunk
	Inputs       []V2Input
	DeleteInputs []string
	Limits       V2Limits
}
