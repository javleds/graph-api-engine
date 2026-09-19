# API Execution Engine v2: resultados progresivos y ejecución reanudable

## Contexto

V1 ensambla el resultado completo en memoria y lo publica al terminar. Su límite de 16 MiB protege ese modelo. SQLite persistido por sí solo no reduce esa memoria: v2 debe guardar, confirmar y liberar fragmentos durante el trabajo.

## Objetivo

Ejecutar el mismo lenguaje de grafos con memoria acotada, entregar resultados mientras el job corre y reanudar avances confirmados después de reiniciar. Objetivo de capacidad: 1 GiB publicado por ejecución; no es RAM reservada ni consumo mínimo.

## Alcance

API `/v2`, cola durable por ejecución, fragmentos de árbol inmutables, proyección consultable, recuperación, exportación NDJSON, webhook terminal y soporte en playground. SQLite sigue siendo una única instancia con volumen persistente y bloqueo exclusivo.

## Fuera de alcance

SSE, WebSockets, múltiples instancias, pausa manual, exportación de un JSON gigante reconstruido y exactamente una llamada externa ante caídas. Una petición en vuelo puede repetirse; una página confirmada no debe volver a solicitarse.

## Comportamiento esperado

Los datos confirmados aparecen antes de completar descendientes. El cliente distingue datos disponibles de ejecución completa. Cada fragmento tiene secuencia decimal representada como string; repetir su aplicación no duplica nodos. El orden de llegada no determina el orden de arrays. Una desconexión no detiene el job. Los fragmentos confirmados permanecen disponibles aunque el resultado termine con errores o falle.

## Decisiones técnicas

Mantener `definition.version: 1`: la ruta identifica el protocolo de ejecución. V1 conserva contrato, ensamblador, límites y recuperación PROCESS_INTERRUPTED. V2 persiste versión de checkpoint, deadline absoluto, generación, contadores, tareas y entradas aceptadas cifradas. Procesar trabajo acotado; nunca cargar el resultado completo o todas las tareas. Reutilizar fetch, cuotas, cache, singleflight, egress, autenticación y outbox. Checkpoints y publicación forman una transacción. Identificadores deterministas y CAS impiden duplicados tras recuperación.

## Componentes afectados

Execution/domain/application/sqlite, worker, bootstrap, API HTTP, notificaciones, configuración y assets de playground. Interfaces pequeñas en los límites de persistencia; no imports de infraestructura en aplicación ni de worker en HTTP.

## Backend

Admitir v2 con namespace de idempotencia independiente y persistir su modo antes de despachar. Guardar cada página aceptada antes de enriquecerla, luego publicar en lotes limitados con cursor interno. Descubrir hijos progresivamente y conservar referencias de contexto de ancestros. Preservar selectores, normalización escalar, colisiones, páginas y resultados parciales de v1. Reservar intentos HTTP durables antes de enviar, incluso si el proceso cae después de reservar. No reiniciar deadline ni presupuestos al recuperar. Detener avance y marcar readiness no saludable ante fallos de persistencia; no confirmar datos sólo en RAM.

## Frontend

Reutilizar editor, validación, diagnóstico HTTP, estilos y visor existente. Selector v1/v2, v1 inicialmente. V2 muestra estado del job separado de conexión, cursor, bytes, fragmentos y recuperaciones. Árbol lazy con hijos paginados, pestaña de fragmentos con ventana máxima de 200, raw readonly del fragmento seleccionado y descarga NDJSON. Actualización por polling secuencial de 1 s; cancelación y generación evitan respuestas obsoletas. No reconstruir 1 GiB en JavaScript. Mantener diseño desktop de dos columnas y mobile apilado.

## Persistencia

Migración aditiva sin modificar la inicial: modo default v1 para filas anteriores; estado v2 con deadline/generación/checkpoint/counters; tareas con clave determinista, revisión y prioridad; entradas intermedias cifradas; log de fragmentos y proyección indexada. Índices para ejecución/secuencia, tareas pendientes y parent/posición. Cifrar payloads y nombres/rutas sensibles; usar IDs opacos en índices. Confirmar fragmentos, proyección, checkpoint y tareas descubiertas en una sola transacción. Terminal, fragmento terminal y outbox también atómicos. Aplicar retención de resultados a fragmentos/proyección, conservar metadata más tiempo. Guardar respaldo del volumen y llaves antes de migrar una instalación; rollback requiere restaurar respaldo, no ejecutar el binario anterior sobre checkpoints v2.

## API

- `POST /v2/executions`: contrato de definición existente; webhook.payload sólo status (default), result retorna 422.
- `GET /v2/executions/{id}`: estado, progreso, recuperación, resumen y links.
- `GET /v2/executions/{id}/chunks?after=0&limit=100`: cursor exclusivo, respuesta con nextCursor, hasMore, terminal y lastSequence; máximo 100 por defecto y 1 MiB por respuesta.
- `GET /v2/executions/{id}/tree?parentId=…&after=…&limit=…`: hijos inmediatos paginados, sin descargar el árbol completo.
- `GET /v2/executions/{id}/export`: NDJSON con cabecera del corte capturado al iniciar y cierre verificable; permite snapshot de job en curso. Timeout de inactividad de escritura por ruta.
- Equivalentes `/playground/api/v2`, incluyendo validación sin efectos.

Chunk versionado con executionId, sequence string, timestamp y tipo data/error/terminal. Operaciones data: declarar object/array, publicar scalar (incluye null), cerrar nodo. Nodo con ID estable, parentId, key o index y ruta lógica. Declarar contenedor antes de hijos, validar colisiones antes de publicar, preservar literales numéricos. Un escalar indivisible que exceda 256 KiB con envelope causa RESULT_VALUE_TOO_LARGE. Terminal incluye estado, resumen, roots y secuencia final. Un lote vacío no implica terminal.

## Validaciones

Cursor decimal canónico; cursor mal formado o por delante del último retorna 422, otro owner o modo retorna 404 y resultado expirado 410. Valores y presupuestos positivos y compatibles. Valores por defecto v2: 1 GiB publicado por ejecución; 8 GiB lógicos globales incluyendo entradas, tareas y proyección; un millón de valores y fragmentos; 100 mil tareas pendientes; máximo cuatro tareas activas por ejecución dentro del límite global. Tamaño máximo de chunk 256 KiB. Reservar capacidad para cierre, no borrar datos activos para continuar. Se conservan límites de respuesta upstream, profundidad, peticiones, intentos, deadline y disco libre. Los límites de bytes son capacidad almacenada, no asignaciones de memoria.

## Permisos y seguridad

Mismos scopes y aislamiento de owner. Playground conserva identidad local compartida y protección de origen en mutaciones. Las rutas de una versión no exponen jobs de otra. Credenciales y datos persistidos cifrados. Visor usa textContent, raw sin reinterpretación y enlaces del webhook acordes al owner/protocolo. Webhook sólo notifica al final con resumen, expiry, última secuencia y links; conserva firma/reintentos/outbox.

## Casos límite

Caída antes/después de aceptar página, checkpoint, descubrir hijos y publicar terminal; deadline vencido durante parada; presupuesto agotado; scalar enorme; paginación repetida; errores parciales; checkpoint incompatible; disco lleno; conexión lenta; petición externa en vuelo. Recuperar mismo ID, invalidar generación previa y no repetir trabajo confirmado. Un fallo técnico no elimina fragmentos previos.

## Estrategia de pruebas

Comparar replay de fragmentos con fixtures v1 (raíces, descendientes, escalares, ancestros, paginación, colisiones y errores). Verificar publicación antes del fin, orden y duplicados, matriz de caídas, persistencia de intentos/deadline, aislamiento, expiración, límites y finalización/outbox atómicos. Navegador desktop/mobile: reconexión, números grandes, XSS, ventanas acotadas y árbol lazy. Exportación válida con corte estable. Fixture CI mayor a 16 MiB y aceptación dedicada 1 GiB con recuperación, medición de heap/disco/latencia. Techo de heap vivo 512 MiB para fixture controlado de una ejecución, no promesa de consumo habitual.

## Estrategia de validación

Tests enfocados y arquitectura por etapa; cierre con make check, browser tests, build/container y make test-v2-large. No afirmar capacidad o recuperación sin pruebas ejecutadas. Registrar mediciones y limitaciones operativas.

## Estrategia de commits

1. `docs(v2): define resumable chunk protocol and acceptance criteria`
2. `feat(storage): persist v2 chunks checkpoints and tree projection`
3. `feat(execution): admit versioned v2 jobs with durable budgets`
4. `feat(worker): checkpoint fetched inputs and resume v2 tasks`
5. `feat(worker): publish bounded tree fragments incrementally`
6. `feat(execution): finalize resumable results and terminal webhooks`
7. `feat(api): expose v2 chunk cursors tree browsing and NDJSON export`
8. `feat(playground): browse live v2 trees and fragments`
9. `test(v2): verify crash recovery compatibility and gigabyte results`
10. `docs(ops): document v2 limits recovery and deployment`

Cada commit conserva una responsabilidad revisable y se valida con checks relevantes. Este documento es la fuente de verdad; evitar duplicar requisitos en planes paralelos.

## Archivos estimados a modificar

Módulos indicados arriba, migración engine 000002, contracts y guía del request, configuración de ejemplo/defaults, Makefile y pruebas de integración/browser.

## Riesgos

Amplificación de disco por cifrado/índices/WAL y checkpoints; latencia por commits; respuestas upstream grandes; restauración requiere llaves. El heap depende de tamaño de páginas y concurrencia, no sólo del resultado acumulado. El volumen persistente es necesario para recuperación, pero no sustituye backups ni límites de disco.

## Preguntas abiertas

Ninguna decisión de producto pendiente. Los detalles de tipos y nombres de tablas se documentan junto a contratos y código. Toda desviación material de los criterios anteriores debe quedar explícita y validada.

## Plan de implementación

1. Fijar este contrato y criterios; revisar paths v1 y tests existentes.
2. Añadir tablas/tipos/puertos y transacciones de checkpoints; comprobar CAS, cifrado, límites y lectura paginada.
3. Añadir modo/versionado, idempotencia, configuración y presupuesto durable; probar compatibilidad v1.
4. Runner de tareas persistidas y entrada aceptada; comprobar recuperación sin refetch confirmado.
5. Publicación incremental y semántica completa de árbol/paginación; comparar fixtures v1.
6. Cierre atómico, errores, retención, webhook y bootstrap/recovery; comprobar reinicio y outbox.
7. Endpoints y exportación acotada con autorización/cursor/expiry; probar HTTP real.
8. UI incremental reutilizando componentes visuales; pruebas desktop/mobile.
9. Fallos controlados, regresiones y fixture grande medido; ejecutar checks completos.
10. Documentar operación, contratos, recursos medidos y despliegue con respaldo.
