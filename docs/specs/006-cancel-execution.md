# Cancelar ejecuciones desde el playground y API

## Contexto

Detener seguimiento sólo desconecta la UI. El usuario requiere detener en Go el árbol activo v1/v2. Los runners ya propagan context a fetch; la publicación terminal existente es durable y atómica con el webhook y el terminal v2.

## Objetivo

Botón independiente **Abortar ejecución** que solicite cancelación real, corte las llamadas del job y conserve un cierre durable.

## Alcance

API pública y rutas playground v1/v2, caso de uso de cancelación, señal por ejecución en dispatcher, UI y pruebas.

## Fuera de alcance

Deshacer efectos ya producidos en proveedores, cancelar otros jobs o reanudar un job abortado. No cancelar operaciones compartidas de singleflight mientras otro consumidor las necesite.

## Comportamiento esperado

Cancelar pending/running publica failed con error EXECUTION_CANCELLED. UI muestra Cancelada. No ampliar enum de estados ni modificar significado de detener seguimiento. Chunks confirmados v2 quedan disponibles con terminal de cancelación; v1 no ofrece resultado de un job fallido. Cancelación repetida o posterior a terminal devuelve el estado final existente, sin duplicar webhook. Si finalización natural gana la carrera, se conserva. No se afirma cancelación si almacenamiento falla.

## Decisiones técnicas

Persistir terminal antes de señalizar context cancel al dispatcher. El commit terminal es el punto de decisión: impide nuevos checkpoints v2 y recuperación. Después de publicar, cancelar todos los context registrados para el ID. Registrar context síncronamente antes de lanzar goroutine evita pérdida de señal en jobs todavía pending. Usar CAS/reintentos acotados para carreras con claim/finalización/chunks. Finalizador existente prepara webhook; report y progreso confirmado v2 se toman del checkpoint cuando existe. No mantener un flag en RAM como evidencia de cancelación. Un request cancel cuyo cliente se desconecta puede completarse en una ventana de persistencia acotada, nunca en background ilimitado.

## Componentes afectados

Execution/application e inprocess, API/http, bootstrap y playground assets/tests. Mantener arquitectura: HTTP no importa worker ni infraestructura.

## Backend

Caso de uso CancelExecution verifica owner/version antes de modificar, finaliza con error explícito y señala ExecutionCanceller. Dispatcher mantiene registros por ID (incluyendo múltiples reservas del mismo ID) con limpieza al salir; no retiene IDs terminados. Supervisor ignora errores de runners de jobs ya terminales antes de marcar readiness unhealthy. Conservar aislamiento de shutdown y cancelación de otros jobs.

## Frontend

Reutilizar botones/status/debug/editor existentes. Abortar habilitado sólo para job conocido pending/running, incluso si seguimiento está detenido. Deshabilitado mientras envía o tras terminal. POST según versión; seguir consultando estado y recuperar fragmentos v2. Mostrar error HTTP sin afirmar éxito, permitir retry. Proteger respuestas tardías al cambiar job/versión con generación y AbortController. No confirmación modal adicional.

## Persistencia

No nueva migración: failed + error EXECUTION_CANCELLED usa el estado terminal durable actual. Retención configurada para chunks v2. Sin borrado de resultados previos; carreras nunca sobrescriben un terminal existente.

## API

POST /v1/executions/{id}/cancel, /v2/executions/{id}/cancel y equivalentes playground. Sin body. 200 devuelve id/status/error y links cuando terminal confirmado (cancelado o terminal previo). Owner o versión incorrecta404; falta permiso403; almacenamiento503. Requiere executions:create y executions:read, ambos permisos existentes, para crear cambios y leer respuesta. Playground conserva protección CrossOrigin en POST.

## Validaciones

Versiones y owner comprobados en caso de uso, no sólo UI. Peticiones repetidas idempotentes. Finalización bounded por timeout de persistencia; conflicto CAS reconsulta terminal y vuelve a intentar sin bucle ilimitado.

## Permisos y seguridad

No acceso entre owners. No secretos en logs o nuevos responses. Señal sólo se envía después de verificar dueño y confirmar finalización. Pruebas cross-origin e insuficientes permisos.

## Casos límite

Pending antes de iniciar goroutine, running con HTTP bloqueado, job terminado, doble click/retry, finalización concurrente, desconexión UI, reinicio, fallo de persistencia, otros jobs consumidores de singleflight.

## Estrategia de pruebas

Dispatcher: cancela sólo ID elegido y libera slots/context; registration antes de run. Aplicación: owner/version/terminal/CAS/errorpersistencia. HTTP: endpoints públicos/locales y permisos/origen. Integración: upstream bloqueado recibe cancelación v1/v2, no descendientes nuevos, readiness sigue healthy, v2 chunks conservados y no resume tras restart. Browser desktop/mobile: botón, POST versión correcta, terminal Cancelada, error retry y distinción stop.

## Estrategia de validación

Tests enfocados por etapa, make check y navegador; build y despliegue local preservando volumen. Actualizar documentación de cancelación y limitación de peticiones ya enviadas.

## Estrategia de commits

1. docs: specify server-side execution cancellation
2. feat(execution): cancel jobs durably and stop active work
3. feat(playground): abort v1 and v2 executions on the server

## Archivos estimados a modificar

Módulos anteriores; nueva spec006, guía v2 y README. Tests unitarios, HTTP, integración y browser.

## Riesgos

No se pueden deshacer acciones externas ya ejecutadas. Una operación HTTP compartida puede continuar para otro job; la rama cancelada no sigue procesando. Fallo de persistencia no garantiza cancelación y debe ser visible. La respuesta confirma terminal durable, la liberación de transporte es cooperativa inmediata mediante context.

## Preguntas abiertas

Ninguna; se conserva el contrato de estados usando código de cancelación explícito.

## Plan de implementación

1. Caso de uso, puertos y registro dispatcher; probar aislamiento y persistencia/CAS.
2. Rutas autenticadas/locales y wiring bootstrap; probar cancelación contra upstream real bloqueado y cierre v2.
3. Botón y estado UI con control de sesiones; pruebas desktop/mobile y documentación. Verificar checks y servicio local, registrar commits por etapa.
