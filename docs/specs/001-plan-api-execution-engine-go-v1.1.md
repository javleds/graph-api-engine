# Plan de desarrollo incremental — API Execution Engine Go v1.1

## Fuente de verdad

Implementar íntegramente [el spec v1.1](spec-api-execution-engine-go-v1.1.md). Se conservan arquitectura, estructura, contratos y secuencia de implementación. El spec contiene los detalles normativos; este documento agrega trazabilidad de commits sin duplicarlos. Punto inicial: repositorio sin implementación ni commits. Crear directamente las migraciones v1.1, sin migración desde v1.0.

## Secuencia de commits


Cada commit debe compilar y pasar las validaciones existentes. Los commits deben ser revisables por responsabilidad; no dejar toda la funcionalidad en un único cambio ni añadir módulos Redis vacíos para aparentar cobertura del futuro.

| # | Commit sugerido | Entregable revisable |
|---|---|---|
| 01 | `docs: define engine contracts and architecture` | Este spec en `docs/specs`, ADRs y fixtures JSON. |
| 02 | `chore: bootstrap Go module and architecture checks` | `cmd`, carpetas, Makefile, reglas un tipo/archivo, versiones fijadas y CI básica. |
| 03 | `feat: implement definition values and validation` | DTOs, schemas, normalización de una/varias raíces, selector, templates, duraciones y pruebas. |
| 04 | `feat: add secure SQLite persistence` | Migraciones, pools, cifrado, instance lock, repositories y guard de versión. |
| 05 | `feat: expose execution HTTP API` | Auth de consumidores, creación/idempotencia, envelope de status/result y errores. Dispatcher fake en tests. |
| 06 | `feat: run bounded local executions` | Dispatcher real, supervisor, contextos y recorrido secuencial correcto. |
| 07 | `feat: implement safe upstream HTTP transport` | URL builder, egress, timeouts, límites de body y clasificación de respuestas. |
| 08 | `feat: enforce shared hierarchical throttling` | Scopes, gate atómico, pacing, cooldown, persistencia y tests de concurrencia. |
| 09 | `feat: add SQLite response cache behind port` | Frescura, keys aisladas, TTL, políticas HTTP y limpieza. |
| 10 | `feat: coalesce in-flight requests safely` | Adaptador local, lifecycle de subscribers y respuestas inmutables. |
| 11 | `feat: enrich paginated trees with bounded concurrency` | Paginación parcial, fallback inline, presupuesto, orden, aislamiento de nodos y varias raíces. |
| 12 | `feat: finalize executions and recover interrupted state` | Resultado atómico, outcomes parciales, informe, recuperación y shutdown. |
| 13 | `feat: deliver authenticated durable webhooks` | Outbox, tres tipos de evento terminal, límites de payload, auth, HMAC, retries y firma. |
| 14 | `feat: add operational limits and observability` | Config completa, progreso, métricas, retención, disco y salud. |
| 15 | `test: add end-to-end failure and contract suites` | Matriz integral, fuzzing y simulación de caídas. |
| 16 | `build: package single-instance engine container` | Dockerfile, Compose, permisos, README y ejecución reproducible. |

Las funciones incompletas de commits intermedios no se anuncian como soportadas en producción. El orden de algunos commits puede ajustarse para mantener el build verde, pero las responsabilidades de revisión no deben fusionarse sin motivo.

Si la implementación del borrador 1.0 ya comenzó, aplicar la revisión en commits separados: (a) contratos/ADRs y fixtures de parciales, (b) migración de estados/eventos/informe, (c) clasificación tipada de fallos, (d) aislamiento y collector, (e) paginación/varias raíces, (f) API/webhook y (g) regresión end-to-end. No cambiar la lógica del worker sin actualizar también DTOs, SQL y webhooks.

Antes de cerrar cada commit: revisar que no haya secretos, actualizar el spec si cambia una decisión, ejecutar validación determinista, agregar tests que fallen sin el cambio y verificar tamaño/cohesión de archivos.


## Referencias y validación por paso

| Paso | Secciones normativas | Validación específica |
|---|---|---|
|01|3,7,9,27,30,34|JSON de ejemplos, contratos y ADRs referenciados.|
|02|9,24,26.8|Build, checker AST positivo/negativo y CI.|
|03|3–6,29–31,34|Contrato estricto, defaults, precisión, templates, roots y validación offline.|
|04|10,18,20|SQLite temporal, versión real, pragmas, lock, cifrado, transacciones y CAS.|
|05|7,11,29|Auth/owner, replay concurrente, saturación, errores y envelopes.|
|06|10–12,17|Reservas, contextos independientes del POST, fallos locales y panic.|
|07|5,13,17,20|HTTP/TLS controlado, SSRF, límites, clasificación y retries.|
|08|14,26.4|Cuota conjunta, pacing, cancelación, cooldown durable y cada intento.|
|09|10,15,26.5|TTL, aislamiento, degradación SQL, cero admisión para hits.|
|10|10,13,16,26.5|Followers, cancelación, generaciones, inmutabilidad y cobro físico.|
|11|4,6,12,17,26.3,26.7|Orden, prefijos, presupuesto, varias raíces y un slot sin deadlock.|
|12|10.6,17,18,22|Atomicidad terminal, truncamiento, shutdown y recuperación sin reejecución.|
|13|19,26.6|Firma exacta, retries, duplicados, payload y reinicio.|
|14|21–23|Config estricta, límites, métricas, salud, retención y disco.|
|15|26,28,34.1|Matrices C/P/T/I/E, fuzzing, carreras, caídas y carga acotada.|
|16|24|Contenedor no root, permisos, CLI, persistencia y smoke completo.|

## Disciplina y cierre

Cada paso depende de los anteriores e incorpora sus pruebas. Los puertos y fakes permiten integrar capacidades pendientes sin anunciar soporte productivo. Los límites se implementan junto al comportamiento protegido, completando integración operativa en paso 14. Los contratos públicos siguen §§3/7/29/30/34 y las interfaces internas §10.

Cada commit de código debe pasar formato, go vet, go test, go test -race y checker AST. CI verifica formato sin modificar y ejecuta govulncheck fijado. Validar compatibilidad real de toolchain y dependencias antes de fijar versiones. No consumir APIs reales ni secretos en tests. Usar SQLite temporal, reloj inyectable y servidores fake. No alterar checksums de migraciones ya incorporadas.

Revisar diff, secretos, imports, cohesión y archivos seleccionados antes de cada commit. Registrar validaciones y limitaciones temporales en su cuerpo. Terminar únicamente con todos los criterios de §28 demostrados y trazabilidad de §26.

## Registro de implementación

Se conservaron los 16 hitos. El paso 06 incorpora dispatcher y supervisión; el recorrido concreto se incorpora junto con la composición del paso 11 para mantener límites coherentes. El paso 04 incluye la primitiva SQL de finalización atómica y el 12 su orquestación y recuperación.

Entre los pasos 14 y 15 se agrega `fix(throttle): recover failed cooldown initialization`: una lectura inicial cancelada o fallida puede reintentarse sin reiniciar el proceso ni perder pacing/cooldown. Incluye reproducción y regresión concurrente. El historial resultante contiene 17 commits convencionales.

El catálogo del spec incorpora `INTERNAL_WORKER_ERROR` para fallos técnicos distintos de panic. La evidencia y los comandos de aceptación están en [docs/testing.md](../testing.md); la operación y configuración, en [README.md](../../README.md).
