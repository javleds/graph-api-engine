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

La respuesta 202 contiene `id`, `status` y links `self`, `chunks`, `tree`, `export`. Un replay idempotente terminal devuelve 200. V1 y v2 tienen namespaces de idempotencia independientes. Un ID v2 consultado mediante v1 devuelve 404, y viceversa. Se mantienen autorización y aislamiento por owner.

Consultar `GET /v2/executions/{id}` para estado, errores, raíces, progreso y `v2`: última secuencia, bytes publicados/almacenados, deadline, recuperaciones y presupuestos consumidos. Los estados conservan `pending`, `running`, `completed`, `completed_with_errors`, `failed`. La metadata puede seguir disponible después de expirar los fragmentos.

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

Conservar literales numéricos: JSON.parse convencional redondea enteros mayores a 2^53−1. Usar un parser que preserve números, o mantener el texto raw. `null` es un valor definitivo; un contenedor pendiente todavía puede recibir hijos. Un fallo técnico puede dejar contenedores pendientes en fragmentos previos: el terminal marca el resultado global como incompleto/fallido. Nunca interpretar la ausencia de `close` como éxito.

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

V2 emite un webhook terminal con firma/reintentos existentes, estado, resumen, `lastSequence`, `resultExpiresAt` y links de consulta. No envía un webhook por chunk ni incluye el resultado completo. Usar los links con las credenciales del mismo owner. Los jobs del playground enlazan sus rutas locales `/playground/api/v2`.
