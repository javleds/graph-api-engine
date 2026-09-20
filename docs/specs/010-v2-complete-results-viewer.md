# V2: cadenas terminadas, eventos y viewer estable

## Contexto y objetivo

V2 publica fragmentos antes de cerrar sus descendientes. Añadir una unidad durable de resultado: una raíz single completa o cada elemento de una colección raíz con todos sus descendientes. El cierre admite completed, completed_with_errors y failed. Conservar el recorrido secuencial y los chunks existentes.

## Alcance y comportamiento

Confirmar resultado, evento node.finished, chunks y checkpoint atómicamente. La API incremental y el suscriptor webhook consumen el mismo registro sin eliminarlo. Una cadena interrumpida no constituye un resultado terminado. Colecciones vacías se explican mediante el terminal.

## Decisiones técnicas y backend

Persistir identidades estables, secuencia propia y referencias al rango de chunks, sin duplicar subárboles. Liberar referencias del worker después de confirmar; mantener inputs todavía necesarios. Reconstruir JSON mediante lecturas acotadas y una pila de serialización. Las nuevas capacidades sólo aplican a nuevas ejecuciones; las anteriores conservan su comportamiento.

## Persistencia

Migración 000003 aditiva para resultados/eventos, avance del suscriptor y capacidad por ejecución. Adaptar la outbox para múltiples node.finished por ejecución conservando unicidad de eventos y terminales. Cifrar metadata sensible, contabilizar bytes y aplicar retención. Respaldar volumen y claves antes de migrar; rollback mediante restauración.

## API

GET /v2/executions/{id}/results?after=0&limit=100 lista resultados definitivos con eventId y cursor decimal string independiente de chunks. GET /v2/executions/{id}/results/{nodeId} transmite el envelope y JSON completo. Añadir equivalentes playground. Mantener scopes, owners, 404/410/422 y precisión numérica. Exponer estado paginado de entregas y resumen separado del webhook terminal.

## Webhooks

Sin cambiar el request, las nuevas ejecuciones v2 con webhook envían cada cadena y el aviso terminal. V1 conserva su contrato. Un suscriptor durable prepara entregas independientemente del worker. Congelar el payload y avanzar su cursor atómicamente con la entrega. Reutilizar firma, credenciales, reintentos y egress. maxPayloadBytes (256 KiB por defecto) incluye envelope: superar el límite registra WEBHOOK_PAYLOAD_TOO_LARGE sin enviar HTTP ni truncar el resultado. La API conserva acceso al contenido completo.

Una cola llena retrasa el suscriptor, no directamente al worker. Respetar presupuestos globales, reservas terminales y deliveryMaxAge desde el evento. Retención de resultados debe cubrir esta ventana. Entrega potencialmente duplicada y fuera de orden; deduplicar por eventId. Terminal incluye total y última secuencia de resultados y no promete recepción de webhooks.

## Frontend y componentes reutilizados

Reutilizar editor, estilos, visor JSON, validación y controles del playground JavaScript. Vista Resultados con páginas estables de 50 entradas, detalle independiente, aviso de nuevos resultados sin sustituir DOM/foco/scroll/expansión. Primer lote automático. Un detalle en memoria; más de 1 MiB se explora por ramas o descarga. Chunks y árbol incremental permanecen en Diagnóstico. Distinguir procesamiento y notificaciones.

## Validaciones, permisos y seguridad

Conservar autorización por owner, cifrado, XSS mediante textContent, literales numéricos exactos y cancelación HTTP. Cursores canónicos, consultas acotadas y límite de payload medido sobre bytes JSON finales.

## Casos límite y pruebas

Probar single, colección, hijos fallidos, vacíos, paginación, null, números grandes, cancelación, caída antes/después del commit, duplicados, ACK perdido, exceso de tamaño, cola llena, receptor lento, expiración, aislamiento y recuperación. UI: ráfagas, foco y selección estables, navegación y respuestas tardías. Medir memoria con resultados y notificaciones grandes.

## Plan, validación y commits

1. Resultados/eventos durables y worker; validar dominio, SQLite y recuperación.
2. API y serialización acotada; validar contratos HTTP y precisión.
3. Suscriptor/outbox/dispatcher; validar capacidad, tamaño, reintentos y recuperación.
4. Viewer; validar desktop/mobile y estabilidad.
5. Documentación y validación integral: make check, make playground-test, make build y make test-v2-large.

Separar commits por esas responsabilidades, incluyendo pruebas en cada bloque. No introducir broker, nuevas dependencias de producción, SSE, multirréplica ni migración del playground a Vue.

## Componentes y archivos estimados

Worker, execution/domain/application/sqlite, notification/application/sqlite, HTTP y DTOs, bootstrap, configuración, migraciones engine, assets y pruebas del playground, guías v2.

## Riesgos y preguntas abiertas

Mayor disco por outbox; receptores pueden estar caídos hasta vencer el plazo; cambios intencionales en webhooks de nuevas ejecuciones v2. No garantizar orden HTTP ni entrega exitosa. Ninguna decisión de producto pendiente.

## Evidencia de implementación y validación

Implementado el 2026-09-20. API de resultados completos y diagnóstico de entregas, suscriptor durable, límite estricto de mensajes, entrega independiente por slots y viewer estable. La migración conserva entregas anteriores y no activa eventos retrospectivos. Assets web regenerados para incluir la guía actualizada.

Validación: suite Go completa y detector de carreras, go vet, build, typecheck web y 62 pruebas de navegador desktop/mobile. Integración HTTP real comprueba single y primer elemento enriquecido mientras el siguiente sigue bloqueado, misma identidad por API/webhook y cierre con tres resultados. Pruebas adicionales cubren rollback de evento/checkpoint, recuperación, migración con datos previos, tamaño, capacidad y JSON exacto.

Fixture de capacidad: 1.071.702.456 bytes publicados, 1.024 páginas, una recuperación y 8.192 resultados sin duplicación; pico HeapAlloc muestreado 19.964.120 bytes, disco 2.898.964.928 bytes y 27,997 s. Fixture separado de una única cadena de al menos 128 MiB: lectura completa a un writer sin acumulación, pico HeapAlloc muestreado 8.025.760 bytes. Son mediciones de fixtures locales controlados, no garantías de RSS ni de latencia de proveedores reales.

`make check` se detiene en reglas preexistentes del chequeo de arquitectura: dos tipos en security_config.go, dos tipos en playground_session.go y un modelo anónimo en egresspattern/pattern_test.go. También existe formato pendiente previo en egress_config.go. Estos archivos no se modificaron; los checks Go, race, vet y build se ejecutaron por separado. Ninguna infracción nueva de arquitectura o formato permanece en los archivos cambiados.
