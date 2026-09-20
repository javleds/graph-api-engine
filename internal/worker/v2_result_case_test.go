package worker

type v2ResultCase struct {
	name, body, status string
	child              bool
	fetchFailure       bool
}
