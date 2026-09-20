package worker

import execution "github.com/javleds/api-engine/internal/execution/domain"

// A single atomic scheduler checkpoint contains only the bounded active frontier.
// Finished tasks are removed after their parent join, never accumulated here.
type v2ParallelCheckpoint struct {
	Cursor          int                            `json:"cursor"`
	Version         int                            `json:"version"`
	Tasks           []*v2ParallelTask              `json:"tasks"`
	Report          execution.ExecutionErrorReport `json:"report"`
	CompletedReport execution.ExecutionErrorReport `json:"completedReport"`
	Progress        execution.ExecutionProgress    `json:"progress"`
	RootDone        []bool                         `json:"rootDone"`
	NextIndex       []int64                        `json:"nextIndex"`
	FinishedIndices []map[int64]bool               `json:"finishedIndices"`
}
