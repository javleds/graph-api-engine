# Consumir ejecuciones v2

V2 procesa el grafo por pasos y publica fragmentos confirmados en SQLite. El resultado completo no se mantiene en RAM. El request usa el mismo lenguaje explicado en [la guía de grafos](api-request-guide.md), con `definition.version: 1`, y se envía a `POST /v2/executions`. El [schema v2](../contracts/create_execution_v2.schema.json) restringe `webhook.payload` a `status`; omitirlo también selecciona status.

## Crear y seguir

```sh
curl -X POST http://localhost:8080/v2/executions \
  -H "Authorization: Bearer $ENGINE_API_TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: mi-grafo-001' \
  --data-binary @request.json
```

La respuesta 202 contiene `id`, `status` y links de consulta. Las nuevas ejecuciones también anuncian resultados completos mediante `results`. Un replay idempotente terminal devuelve 200. V1 y v2 tienen namespaces de idempotencia independientes. Un ID v2 consultado mediante v1 devuelve 404, y viceversa. Se mantienen autorización y aislamiento por owner.

Consultar `GET /v2/executions/{id}` para estado, errores, raíces, progreso y `v2`: última secuencia, bytes publicados/almacenados, deadline, recuperaciones y presupuestos consumidos. Los estados conservan `pending`, `running`, `completed`, `completed_with_errors`, `failed`. La metadata puede seguir disponible después de expirar los fragmentos.

## Consumir cadenas completas

`GET /v2/executions/{id}/results?after=0&limit=100` lista resultados definitivos. Una raíz `single` entrega un resultado cuando ella y sus hijos terminan. Una raíz `collection` entrega uno por elemento con todos sus descendientes. Las nuevas ejecuciones procesan ramas independientes concurrentemente y publican por finalización: Charizard puede aparecer antes que Bulbasaur, conservando su índice original. Una colección vacía no produce elementos. Una cadena interrumpida por cancelación no se anuncia como terminada.

El listado contiene `results`, `nextCursor`, `hasMore`, `terminal` y `lastSequence`. Cada resultado identifica `eventId`, `nodeId`, raíz, ubicación, secuencia y estado (`completed`, `completed_with_errors` o `failed`). Su cursor es independiente del cursor de chunks. Conservarlo como string decimal y guardarlo después de aplicar el lote; deduplicar por `eventId`. Incluso con `terminal: true`, consumir páginas pendientes hasta `lastSequence`.

`GET /v2/executions/{id}/results/{nodeId}` devuelve metadata, `data`, diagnósticos y `complete: true` al final. El servidor transmite desde disco; este JSON no tiene el máximo de 256 KiB del webhook. Si la transferencia se interrumpe o falta su cierre válido, repetirla. Los datos publicados son inmutables hasta su expiración. Los fallos de hijos mantienen los datos válidos y los diagnósticos; un fallo sin datos se distingue de un `null` exitoso por el estado.

El playground presenta páginas estables de resultados y un detalle seleccionado. La llegada de más resultados no cambia la página leída; un aviso permite actualizarlos. Los resultados grandes pueden descargarse o explorarse por ramas sin cargar todo en memoria. Árbol incremental y chunks permanecen disponibles como diagnóstico.

Estas capacidades aplican a ejecuciones nuevas después de migrar a 000003. Las anteriores conservan su protocolo previo y no generan resultados o eventos retrospectivos.

## Recibir fragmentos durante la ejecución

Consultar `GET /v2/executions/{id}/chunks?after=0&limit=100`. `after` es exclusivo, decimal canónico y string; no convertirlo a Number en JavaScript. Cada respuesta contiene:

```json
{
  "chunks": [],
  "nextCursor": "0",
  "hasMore": false,
  "terminal": false,
  "lastSequence": "0"
}
```

Leer inmediatamente la siguiente página mientras `hasMore` sea true; cuando sea false y `terminal` false, esperar un segundo. Guardar el cursor **después** de aplicar/persistir el lote. Al reconectar, enviar el último cursor confirmado. Aplicar cada secuencia una sola vez; un retry puede devolver fragmentos que ya se recibieron. `terminal: true` indica que el productor terminó, pero todavía hay que consumir hasta `lastSequence` si `hasMore` es true.

Cada respuesta está acotada a 100 fragmentos y aproximadamente 1 MiB incluyendo envelope. Un fragmento tiene como máximo 256 KiB. Cursores inválidos o adelantados devuelven 422; owner incorrecto/ID inexistente 404; fragmentos expirados 410. Un lote vacío no significa ejecución completa.

## Protocolo del árbol

```json
{
  "version": 2,
  "executionId": "uuid",
  "sequence": "1",
  "createdAt": "2026-09-18T00:00:00Z",
  "type": "data",
  "operation": "object",
  "nodeId": "opaque-id",
  "key": "orders",
  "path": "/orders"
}
```

Tipos y operaciones:

| Tipo | Operación | Significado |
|---|---|---|
| `data` | `object` | Declara objeto pendiente, antes de sus propiedades. |
| `data` | `array` | Declara array pendiente, antes de sus elementos. |
| `data` | `value` | Publica un escalar, incluyendo null, en `value`. |
| `data` | `close` | Cierra un contenedor con `status`. |
| `error` | — | `value` contiene el diagnóstico asociado al nodo. |
| `terminal` | — | Estado final; `value` contiene resumen de errores, raíces y fallo global. |

`nodeId` identifica establemente el nodo. `parentId` enlaza con su contenedor; se omite en raíces. Una propiedad usa `key` (si se omite y no hay `index`, la clave es el string vacío); un elemento usa `index` numérico absoluto, incluyendo arrays paginados. `path` usa JSON Pointer con escape `~0`/`~1`. No interpretar IDs ni inferir índices del orden de llegada. El valor JSON de un nodo publicado no se reemplaza: se agregan descendientes y cierre.

Las ramas concurrentes pueden intercalar sus chunks. El padre se declara antes de sus hijos y se cierra después de ellos; no asumir un rango contiguo por cadena. Conservar literales numéricos: JSON.parse convencional redondea enteros mayores a 2^53−1. Usar un parser que preserve números, o mantener el texto raw. `null` es un valor definitivo; un contenedor pendiente todavía puede recibir hijos. Un fallo técnico puede dejar contenedores pendientes en fragmentos previos: el terminal marca el resultado global como incompleto/fallido. Nunca interpretar la ausencia de `close` como éxito.

Un escalar que no cabe en un fragmento falla con `RESULT_VALUE_TOO_LARGE`; no se parte su string en pedazos arbitrarios. El resultado parcial confirmado sigue siendo consultable. Superar presupuestos produce `EXECUTION_BUDGET_EXCEEDED`.

## Árbol bajo demanda

`GET /v2/executions/{id}/tree?parentId=&after=0&limit=100` devuelve raíces. Usar el `nodeId` como `parentId` para explorar hijos inmediatos. La respuesta tiene `nodes`, `nextCursor`, `hasMore`, `terminal`, `lastSequence`. Cada nodo incluye identidad, relación, `kind` (`object`, `array`, `value`), valor escalar cuando aplica y estado. El cursor de árbol es específico del padre y no es intercambiable con el cursor de chunks.

Para refrescar una rama pendiente, volver a consultar desde 0: el cierre puede cambiar el estado de nodos existentes. Paginar hacia adelante sólo descubre más hijos. El playground navega por ramas y conserva como máximo 200 fragmentos recientes; no arma un JSON gigante en el navegador.

## Descargar NDJSON

`GET /v2/executions/{id}/export` devuelve `application/x-ndjson`. Captura un corte fijo al iniciar, incluso si el job sigue corriendo:

1. Línea `type: "export"`, con `lastSequence` del corte y `terminal`.
2. Fragmentos hasta esa secuencia, uno por línea.
3. Línea `type: "export_end"`, mismo corte y `complete: true`.

Sin la última línea, la descarga está incompleta y debe repetirse. Un export completo con `terminal: false` es un snapshot válido de una ejecución todavía en curso. No equivale a un árbol terminado. Para consumo incremental reanudable usar `/chunks`; export es una descarga desde el inicio. El servidor escribe páginas acotadas y permite 30 segundos de inactividad de escritura, sin cargar todo el resultado.

## Webhook

Las nuevas ejecuciones v2 con webhook envían `node.finished` por cada cadena completa, además del aviso terminal. No requiere campos adicionales: `webhook.payload` sigue admitiendo `status`; los eventos de nodo incluyen siempre su resultado. Sin webhook se conserva el mismo listado de resultados por API. La API también permite recuperar notificaciones fallidas cuando hay webhook.

El evento de nodo incluye versión, `eventId`, `executionId`, `nodeId`, secuencia, raíz, ubicación, estado, fecha, `errorSummary`, `errors`, `result`, `complete` y links. `result` corresponde exactamente a `data` del endpoint del nodo, preservando literales numéricos. La firma HMAC y los reintentos existentes se aplican al payload congelado; sus bytes e identidad no cambian entre intentos.

`webhooks.maxPayloadBytes` (256 KiB por defecto) incluye todo el envelope. Si se supera, la entrega termina `failed` con `WEBHOOK_PAYLOAD_TOO_LARGE`, sin HTTP ni truncamiento. Esto no convierte en fallido el nodo: su contenido sigue disponible por API.

La política opcional del webhook se configura en el body:

```json
{
  "url": "https://consumer.example/events",
  "payload": "status",
  "ordering": "completion",
  "maxInFlight": 1
}
```

`ordering` acepta `completion` (default, por finalización) o `source` (posición original de raíces y elementos). `maxInFlight` acepta enteros desde `0`, default `1`, limitado operativamente por `webhooks.maxConcurrent`. Con uno, esperar respuesta `2xx` o fallo definitivo antes de admitir la siguiente entrega; con un valor mayor, permitir varias pendientes de resolución; con `0`, quitar el límite por ejecución. El límite global del servicio siempre permanece activo. Un reintento conserva su lugar en la ventana. `source` con varias entregas ordena la admisión, no la recepción remota. Un `202` confirma recepción, no el fin del procesamiento del receptor. Estos campos se rechazan en V1.

Ninguna política retrasa el procesamiento del engine ni la publicación por API. Esperar una cadena lenta sólo retrasa los webhooks dependientes de su orden. Las esperas se persisten y consumen presupuesto/plazo de entrega.

Puede haber duplicados y entregas fuera de orden. Deduplicar por `eventId`, también al combinar webhook y API. El terminal conserva `lastSequence`, `resultExpiresAt` y links, e incluye `resultCount` y `lastResultSequence` para las nuevas ejecuciones. Con la nueva política se admite después de resolver las entregas de nodo, incluidas las fallidas: no garantiza recepción de todas. Las ejecuciones guardadas antes del cambio conservan su despacho anterior y no adquieren defaults retroactivamente.

Consultar el resumen de notificaciones de nodo en el estado de ejecución y `GET /v2/executions/{id}/webhooks?after=0&limit=100` para entregas materializadas, intentos y errores. El cursor corresponde a la secuencia de resultados; una entrega puede cambiar de estado, por lo que volver a consultar su página permite revisar reintentos. El resumen incluye eventos pendientes de entrar en la outbox. El campo histórico `webhook` sigue describiendo únicamente el aviso terminal.

Usar los links con las credenciales del mismo owner. Los jobs del playground enlazan sus rutas locales `/playground/api/v2`. V1 y ejecuciones anteriores a 000003 conservan sus webhooks terminales previos.

## Abortar el árbol

`POST /v2/executions/{id}/cancel` solicita la cancelación en Go; no requiere body y exige permisos `executions:create` y `executions:read` del mismo owner. Devuelve 200 con `{id,status,error,links}`. Una cancelación confirmada usa `status: "failed"` y `error.code: "EXECUTION_CANCELLED"`, con terminal y webhook durables. Los chunks previos siguen disponibles hasta su expiración. El job no se reanuda tras reiniciar.

La operación es idempotente: si la finalización natural ganó la carrera, devuelve ese terminal sin sobrescribirlo. Un error de persistencia devuelve 503 y no debe interpretarse como cancelación confirmada. El playground ofrece **Abortar ejecución**, separado de **Detener seguimiento**. No se pueden deshacer peticiones ya procesadas por proveedores; las operaciones compartidas continúan para otros consumidores.
