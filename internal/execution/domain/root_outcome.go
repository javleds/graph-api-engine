package domain

type RootOutcome struct {
	Name          string          `json:"name"`
	Status        ExecutionStatus `json:"status"`
	DataAvailable bool            `json:"dataAvailable"`
}
