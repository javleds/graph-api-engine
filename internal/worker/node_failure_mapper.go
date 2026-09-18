package worker

import (
	"context"
	"errors"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
)

func nodeFailure(call definition.CompiledCall, n NodeContext, code, stage string, attempts int) execution.NodeFailure {
	path, pointer := n.Path, call.Pointer
	failure := execution.NodeFailure{Code: code, Message: failureMessage(code), Scope: "node", NodePath: &path, DefinitionPath: &pointer, Stage: stage, Attempts: attempts, DescendantsSkipped: len(call.Children) > 0}
	if n.Page != nil {
		page := int(*n.Page)
		failure.Page = &page
	}
	return failure
}
func mapFetchFailure(call definition.CompiledCall, n NodeContext, err error) execution.NodeFailure {
	if errors.Is(err, context.DeadlineExceeded) {
		failure := nodeFailure(call, n, "UPSTREAM_TIMEOUT", "fetch", 0)
		failure.Retryable = true
		return failure
	}
	var upstream *fetch.UpstreamFailure
	if errors.As(err, &upstream) {
		failure := nodeFailure(call, n, upstream.Code, "fetch", upstream.Attempts)
		failure.Retryable = upstream.Retryable
		failure.RetryExhausted = upstream.RetryExhausted
		if upstream.HTTPStatus > 0 {
			status := upstream.HTTPStatus
			failure.UpstreamStatus = &status
		}
		return failure
	}
	var invalid *definition.ValidationError
	if errors.As(err, &invalid) {
		return nodeFailure(call, n, invalid.Code, "binding", 0)
	}
	return nodeFailure(call, n, "UPSTREAM_UNAVAILABLE", "fetch", 0)
}
func failureMessage(code string) string {
	switch code {
	case "UPSTREAM_UNAUTHORIZED":
		return "El proveedor rechazó las credenciales de esta llamada."
	case "UPSTREAM_FORBIDDEN":
		return "El proveedor rechazó el acceso a este recurso."
	case "UPSTREAM_NOT_FOUND":
		return "El proveedor no encontró el recurso solicitado."
	case "SELECT_PATH_NOT_FOUND":
		return "No se encontró la ruta de selección en la respuesta."
	case "BINDING_PATH_NOT_FOUND":
		return "Falta un atributo necesario del ancestro."
	case "TYPE_MISMATCH":
		return "El valor no coincide con el tipo de nodo esperado."
	case "PARENT_NOT_ENRICHABLE":
		return "El valor no permite agregar los nodos hijos."
	case "OUTPUT_FIELD_COLLISION":
		return "Un nodo hijo coincide con un atributo existente."
	case "PAGINATION_SIZE_INCONSISTENT":
		return "La página excede el tamaño esperado."
	case "PAGINATION_LIMIT_REACHED":
		return "La paginación alcanzó el límite del servicio."
	case "PAGINATION_NO_PROGRESS":
		return "La página repite los datos de la página anterior."
	case "EXECUTION_TIMEOUT":
		return "La ejecución excedió su tiempo máximo."
	case "EXECUTION_INTERRUPTED":
		return "La ejecución fue interrumpida."
	case "EXECUTION_BUDGET_EXCEEDED":
		return "La ejecución alcanzó su presupuesto de trabajo."
	default:
		return "La llamada no pudo completarse satisfactoriamente."
	}
}
