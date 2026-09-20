# V2 concurrente y entrega configurable

## Contexto y objetivo

V2 recorre una pila secuencial y reconstruye cadenas mediante rangos DFS contiguos. Paralelizar raíces, elementos y descendientes independientes sin perder recuperación, límites ni publicación exclusiva de cadenas completas.

## Alcance y comportamiento

- Nuevas ejecuciones concurrentes; ejecuciones anteriores mantienen su formato y comportamiento.
- Resultados API por finalización, conservando índices originales.
- Webhook V2: `ordering: completion|source` (default completion), `maxInFlight` entero positivo (default 1), limitado por concurrencia operativa global.
- Confirmación mediante cualquier 2xx. La ventana incluye entregas en reintento; éxito, fallo definitivo o vencimiento liberan su lugar. Terminal después de resolver nodos.
- Sin webhook no hay esperas de entrega. V1 no cambia ni acepta las nuevas opciones.

## Decisiones técnicas / backend / persistencia

Tareas durables con dependencias, generación y revisión. Padres esperando no ocupan workers. Expansión acotada; inputs compartidos se conservan hasta su último consumidor. Persistir resultados, evento y cierre en una transacción. Nuevos resultados se reconstruyen desde la proyección de nodos con recorrido paginado; lectores anteriores conservan rangos DFS. Migración aditiva, payloads cifrados y presupuesto de almacenamiento actualizado. Mantener FULL y reservas previas a HTTP.

La implementación conserva una única fila `coordinator`: el checkpoint versión 2 contiene la frontera activa de tareas, sus fases y dependencias. Un coordinador distribuye turnos y confirma cambios atómicamente; las llamadas upstream se ejecutan concurrentemente sobre copias de checkpoints. Esto reutiliza el control de revisión/generación y evita agregar reclamaciones SQL por nodo. Los padres esperando no consumen slots de fetch. Las tareas terminadas se eliminan al incorporarse a su padre. Los índices completados fuera de orden se conservan sólo durante la página upstream activa; la siguiente página espera a que termine la anterior.

El checkpoint guarda un reporte agregado para cancelación y un acumulador separado de raíces finalizadas. Las reservas lógicas se confirman con el checkpoint previo al fetch; cada intento físico conserva su reserva independiente previa a HTTP. Los inputs decodificados son inmutables y pueden compartirse entre consumidores activos. No se garantiza que la concurrencia reduzca costos de CPU o disco cuando todas las respuestas ya están cacheadas.

La migración 000004 añade columnas de formato y orden, diagnósticos por resultado e índices de consultas de entrega. Los defaults de webhook se normalizan sólo en requests nuevos. La identidad idempotente de `completion/1` es equivalente a los campos históricos omitidos, sin modificar snapshots ya guardados.

Para evitar un ciclo de capacidad con `maxPendingDeliveries=1`, la admisión de nodos excluye terminales diferidos que esperan esos mismos nodos. La admisión de nuevas ejecuciones sigue contando reservas y todos los terminales pendientes, acotando ese backlog. El límite HTTP y las ventanas por ejecución no cambian. Las filas persistidas pueden superar el número de slots por terminales diferidos; esta diferencia se documenta operacionalmente.

## API / frontend / validaciones / seguridad

Reutilizar endpoints, autenticación, propietarios, admission gate, caché y políticas de reintentos existentes. Dos nuevos campos de webhook, sin nuevo endpoint. Reutilizar viewer y documentación existente; distinguir procesamiento de entregas pendientes. Nunca inferir orden de recepción con múltiples envíos simultáneos.

## Casos límite y riesgos

Caídas entre reserva y confirmación pueden repetir HTTP, no eventos confirmados. Cancelación invalida escritores anteriores. Errores deben pertenecer a su cadena. Colecciones vacías, fallidas e interrumpidas no bloquean ordering source indefinidamente. Memoria crece con concurrencia y respuestas activas, no con resultado acumulado. No cambiar granularidad pública de chunks ni introducir broker.

## Pruebas y validación

Concurrencia controlada, cadena lenta sin bloquear otra, cuotas/reintentos, worker único sin deadlock, errores aislados, números exactos, recuperación, límites de memoria, payload excedido, cuatro combinaciones de entrega y compatibilidad anterior. Comparar fixtures con caché y latencia; medir engine separado de drenaje de webhooks. Ejecutar Go tests, race, vet, arquitectura, typecheck y pruebas de portal según cambios.

### Validación ejecutada

- Suite Go completa y suite completa con `-race`: pasan. Tras las últimas correcciones se repitieron worker, notificaciones e integración con `-race`.
- `go vet ./...`, typecheck y build del portal: pasan; 64 pruebas Playwright de escritorio/móvil pasan.
- Prueba de aproximadamente 1 GiB con recuperación, liberación de inputs y heap muestreado: pasa. Fixtures y limitaciones de rendimiento documentados en `docs/v2-operations.md`.
- El checker de arquitectura conserva tres infracciones preexistentes en `security_config.go`, `pattern_test.go` y `playground_session.go`; no se ampliaron ni se modificaron esos archivos.
- La revisión añadió regresiones de cancelación con errores parciales, idempotencia histórica y drenaje con capacidad uno. No se desplegó el servicio.

## Plan de implementación y archivos estimados

1. Especificación y pruebas de referencia.
2. Migración y stores en execution/infrastructure/sqlite; lectores compatibles.
3. Scheduler en worker, límites y recuperación.
4. Contrato en definition/dto/schema y dispatcher notification.
5. Documentación y pruebas integrales.

## Estrategia de commits

Commits convencionales: `feat(v2): execute durable branches concurrently`, `feat(webhooks): configure v2 ordering and acknowledgment windows`, `docs(v2): document concurrency and delivery policies`. Persistencia y scheduler se mantienen juntos para no activar checkpoints nuevos sin su runner. Revisar y validar cada bloque antes de confirmar. No desplegar automáticamente. Rollback de datos mediante respaldo, no migración destructiva.

## Preguntas abiertas

Ninguna decisión de producto pendiente. Detalles internos se contrastan con pruebas existentes durante implementación; cualquier desviación material se documentará y consultará.
