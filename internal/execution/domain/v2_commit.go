package domain

type V2Commit struct {
	Progress     *ExecutionProgress
	ID           ExecutionID
	OwnerID      string
	Generation   int64
	Task         V2Task
	NewTasks     []V2Task
	Chunks       []V2Chunk
	Results      []V2Result
	Inputs       []V2Input
	DeleteInputs []string
	Limits       V2Limits
}
