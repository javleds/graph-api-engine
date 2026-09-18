package domain

func ResolveOutcome(roots []RootOutcome, count uint64, failure *ExecutionFailure) (ExecutionStatus, *ExecutionFailure) {
	if failure != nil {
		return Failed, failure
	}
	available := false
	for _, root := range roots {
		available = available || root.DataAvailable
	}
	if !available {
		return Failed, &ExecutionFailure{Code: "ALL_ROOT_CALLS_FAILED", Message: "Ninguna llamada raíz produjo una respuesta válida. Consulte errors para conocer las causas."}
	}
	if count > 0 {
		return CompletedWithErrors, nil
	}
	return Completed, nil
}
