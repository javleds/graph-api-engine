# Playground local de ejecuciones

## Contexto
El servicio Go expone creación, seguimiento y resultado, pero no tiene UI. Se reutilizan los handlers y contratos existentes. La guía de requests y el schema siguen siendo la referencia de input.

## Objetivo
Servir `/playground` para pegar un request, ejecutarlo y explorar su respuesta terminal con estado, progreso y diagnóstico HTTP.

## Alcance
Acceso abierto para uso local; dos paneles (columnas desde 900 px, filas en mobile); input editable; resultado interactivo y raw readonly; copia; depuración de creación/estado/resultado; seguimiento por ID.

## Fuera de alcance
Login, credenciales por .env, aislamiento entre visitantes locales, trazas internas de proveedores, cancelación de jobs, historial persistente y cambios en el motor.

## Comportamiento esperado
Enviar el input original, previa comprobación sintáctica, con Idempotency-Key opcional. Mantener snapshot en memoria. Poll secuencial cada segundo; un solo seguimiento activo. Cancelar consultas obsoletas al cambiar seguimiento. Detener seguimiento no cancela el job. Permitir reanudar por ID. No reintentar POST automáticamente.

Mostrar pendiente, en ejecución, completado, completado con errores y fallido; ID, fechas, duración y contadores existentes. En estados completados consultar resultado; en failed mostrar el estado terminal y su error sin tratar el 409 de resultado como éxito. Errores de red pausan con opción de reanudar. Actualización manual del estado/webhook al terminar.

Raw conserva response.text() sin parsear/reformatear; textarea readonly y copia exacta. Visor de envelope completo con ramas lazy y grupos de 100 elementos. Preservar literales numéricos usando contexto source del reviver JSON.parse; si no existe soporte, ofrecer raw. No interpretar HTML de respuestas.

## Decisiones técnicas
HTML/CSS/JS locales embebidos con go:embed, sin framework ni CDN. Identidad interna fija system:playground compartida entre visitas; reservar ese owner en principals. Ningún token se emite al navegador. Subrouter limitado a creación/lectura reutiliza handlers y devuelve links propios. API /v1 conserva autenticación. No almacenar requests/secretos en navegador.

## Componentes afectados
HTTP server/handlers, adaptador de autenticación, bootstrap, configuración de métricas, assets embebidos y pruebas. No hay UI existente que reutilizar; sí los DTOs, validación, casos de uso y lectores actuales.

## Backend
GET /playground y assets explícitos. Montar solo los tres endpoints de ejecución con autenticador interno; no duplicar todo el router. Rechazar POST cross-origin. CSP restrictiva, nosniff y no-store. Rechazar configuración de metricsPath que colisione con /playground.

## Frontend
Input y acciones ejemplo/ejecutar/limpiar; campo de idempotencia. Debug desplegable para snapshot y respuestas con método, URL, HTTP, duración, X-Request-Id, Location y Retry-After. Capturar bodies no JSON. Accesibilidad por labels, estados textuales y foco visible. Clipboard fallback selecciona raw.

## Persistencia
Sin migraciones. Las ejecuciones usan almacenamiento y retención actuales con owner reservado. No localStorage/sessionStorage ni cookies de credenciales.

## API
POST /playground/api/executions; GET /playground/api/executions/{id}; GET /playground/api/executions/{id}/result. Misma validación/status/body que /v1, excepto prefijo de links. El webhook conserva su contrato existente. No se alteran schemas.

## Validaciones
Sintaxis cliente sin reserializar; validación autoritativa servidor incluyendo duplicados, límites, egress, cuotas y auth upstream. ID validado antes de construir URL en navegador. Detectar soporte de precisión del visor.

## Permisos y seguridad
Acceso abierto explícitamente solicitado para local. Todas las visitas comparten system:playground. No habilitar anónimo en /v1; no permitir que un principal normal use owner reservado. No renderizar HTML externo, no CORS permisivo, solo requests same-origin desde la UI. Configurar el servicio para exposición local; acceso remoto al playground permite crear jobs con esa identidad compartida.

## Casos límite
Replay terminal, input inválido, fallos HTTP/red, 409/410, failed sin resultado, respuestas grandes, IDs >2^53, contenido HTML, resultados tardíos, pausa/reanudación y webhook aún pendiente después de terminar el job.

## Estrategia de pruebas
Go: rutas/assets/CSP, cross-origin, owner aislado, links, auth habitual, configuración/owner reservado. Playwright: layouts desktop/mobile, flujo real con proveedor local, éxito/parcial/fallo, errores/red, detener/reanudar, raw readonly exacto/copia, árbol lazy, precisión y XSS. Mock HTTP para situaciones deterministas y un flujo con engine real.

## Estrategia de validación
Por etapa tests Go focalizados y comprobaciones JS; final make check y suite Playwright. Compilar contenedor y verificar assets embebidos. Dependencias del navegador exclusivas de desarrollo, fijadas en lockfile.

## Estrategia de commits
1. docs: specify local execution playground
2. feat(http): serve playground and scoped execution endpoints
3. feat(playground): submit requests and track execution status
4. feat(playground): add interactive and raw JSON responses
5. test(playground): cover browser workflows and document local usage

## Archivos estimados a modificar
internal/api/http/server.go y handlers; internal/api/http/playground/; internal/security/{domain,infrastructure}/; internal/bootstrap/build_application.go; internal/platform/config/; test/playground/; package.json y lockfile; Makefile; README.md.

## Riesgos
El acceso abierto consume la misma capacidad que la API; se conservan límites. Raw puede contener datos sensibles, se conserva solo en memoria. Browsers sin contexto source usan raw para evitar redondear números. Webhook links mantienen el contrato API existente y requieren credenciales del owner para lectura API; el usuario local puede consultar el job en playground.

## Preguntas abiertas
Ninguna para esta entrega; credenciales configurables se implementarán después.

## Plan de implementación
1. Registrar este documento antes del código.
2. Incorporar shell embebido y subrouter con identidad acotada, links parametrizados, reserva de owner, CSP/origin y pruebas. Verificar que /v1 no cambia.
3. Implementar formulario, snapshots, consultas secuenciales y diagnósticos; validar errores/estados y protección frente a respuestas obsoletas.
4. Incorporar visor preciso lazy, alternancia raw y copia; verificar datos grandes y HTML literal.
5. Agregar pruebas navegador y flujo local real; actualizar README/comandos y comprobar contenedor.

## Evidencia de cierre

Implementados los cinco pasos. `make check` pasó con pruebas normales y detector de carreras. Playwright verificó 26 casos de escritorio/mobile (24 de la suite general y dos regresiones focalizadas de actualización de webhook). Se inspeccionaron capturas de ambos layouts y del visor interactivo. El contenedor sirvió HTML/CSS/JS embebidos y confirmó la separación entre playground abierto y API autenticada.

La revisión corrigió una carrera al actualizar webhook durante la descarga del resultado y una colisión de métricas con paths escapados. Ambas tienen regresión. Las dependencias Playwright son exclusivas de desarrollo y están fijadas en package-lock.json.
