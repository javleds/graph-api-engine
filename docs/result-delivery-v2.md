## V2 · Cada cadena completa, sin esperar al resto

V2 publica resultados definitivos durante la ejecución. Una raíz `single` está lista al terminar ella y todos sus hijos; una raíz `collection` publica cada elemento con todos sus descendientes. La cadena ya publicada no cambia. Las nuevas ejecuciones procesan ramas independientes en paralelo, con concurrencia acotada y respetando cuotas y reintentos. Los resultados se publican por orden de finalización, no por posición en el request.

### Cómo consultar los resultados

1. Envía el request a `POST /v2/executions`. Guarda el `id` y consulta `GET /v2/executions/{id}` para conocer estado y progreso.
2. Lee `GET /v2/executions/{id}/results?after=0&limit=100`, incluso mientras el estado sea `running`.
3. Por cada entrada, sigue `links.result` o consulta `GET /v2/executions/{id}/results/{nodeId}`. Esta respuesta contiene la cadena completa en `data`, sus diagnósticos y `complete: true` al final.
4. Guarda `nextCursor` después de procesar el lote. Si `hasMore` es `true`, lee la siguiente página. Si es `false` y `terminal` es `false`, espera aproximadamente un segundo antes de volver a consultar.

```http
GET /v2/executions/{id}/results?after=0&limit=100
Authorization: Bearer TOKEN_DEL_ENGINE
```

El listado contiene `results`, `nextCursor`, `hasMore`, `terminal` y `lastSequence`. Los cursores son **strings decimales**, no números JavaScript. `after` es exclusivo. Un lote vacío no implica que la ejecución haya terminado. Incluso con `terminal: true`, sigue leyendo mientras haya páginas pendientes.

No hay un `/v2/executions/{id}/result` que reúna todo el grafo. Puedes combinar API y webhook y deduplicar por `eventId`. El resultado completo de un nodo se transmite desde disco; si la respuesta se corta o no contiene un JSON válido con `complete: true`, vuelve a solicitarlo. Preserva los literales numéricos para no redondear identificadores grandes.

### Si incluyes webhook en el request

Sin webhook, los mismos resultados siguen disponibles por API. Con webhook, las nuevas ejecuciones v2 generan automáticamente **un evento `node.finished` por cadena terminada**, además del evento terminal de ejecución. No hace falta activar otra opción y la API sigue disponible.

Este fragmento corresponde al campo `webhook` del request:

```json
{
  "url": "https://consumer.example/events",
  "payload": "status",
  "ordering": "completion",
  "maxInFlight": 1
}
```

En V2, `payload` admite sólo `"status"` o puede omitirse; `"result"` se rechaza con HTTP `422`. Esto no impide recibir los datos: los eventos de nodo incluyen automáticamente la cadena en **`result`**, equivalente a **`data`** en la API del nodo. El aviso terminal contiene el resumen de la ejecución, no todo el grafo.

| Evento | Qué significa | Qué contiene |
| --- | --- | --- |
| `node.finished` | Esta cadena terminó. Su `status` puede ser `completed`, `completed_with_errors` o `failed`. | `eventId`, `executionId`, `nodeId`, `sequence`, estado, `result`, diagnósticos y links. |
| `execution.completed`, `execution.completed_with_errors` o `execution.failed` | El engine dejó de producir resultados. | Resumen, `resultCount`, `lastResultSequence`, `lastSequence` de chunks y links. No confirma que todos los webhooks de nodo hayan llegado. |

#### Orden y concurrencia del receptor

Estos campos son opcionales y exclusivos de V2. Sus defaults se aplican a ejecuciones nuevas; las ya guardadas conservan su política anterior.

| Configuración | Entrega de webhooks |
| --- | --- |
| `ordering: "completion"`, `maxInFlight: 1` (default) | La primera cadena que termina se habilita primero. La siguiente espera confirmación o fallo definitivo de la anterior. |
| `ordering: "completion"`, `maxInFlight: 4` | Hasta cuatro entregas pendientes de resolución, seleccionadas por orden de finalización. |
| `ordering: "source"`, `maxInFlight: 1` | Espera las posiciones anteriores del request y colección; entrega una a la vez. |
| `ordering: "source"`, `maxInFlight: 4` | Admite hasta cuatro entregas en orden original, sin garantizar orden de recepción ni procesamiento remoto. |

`maxInFlight` debe ser un entero positivo. El límite efectivo no supera `webhooks.maxConcurrent` del servicio. Cualquier respuesta **`2xx`**, incluido `202`, confirma recepción; no prueba que el receptor haya terminado su trabajo interno. Mientras una entrega se reintenta, conserva su lugar en la ventana. Al agotar intentos, vencer o fallar definitivamente, permite avanzar a las siguientes.

El orden de webhook **no cambia el paralelismo del engine ni la disponibilidad por API**. Si Bulbasaur tarda 20 segundos y Charizard 2, Charizard aparece por API a los 2; con `source`, su webhook espera a Bulbasaur. La espera es durable, no un grafo acumulado en RAM, pero consume almacenamiento y plazo de entrega.

Puede haber duplicados y entregas fuera de orden. Deduplica por `eventId` y no interpretes el orden HTTP como el orden de la colección. Con la nueva política, el aviso terminal espera a que las entregas de nodo estén resueltas, incluso si terminaron fallidas: no garantiza recepción exitosa de todas. Las ejecuciones antiguas mantienen su comportamiento previo, donde un terminal podía adelantarse a un nodo reintentado. Puedes recuperar lo faltante mediante `/results`.

Consulta `nodeWebhookSummary` en el estado y `GET /v2/executions/{id}/webhooks?after=0&limit=100` para ver entregas, intentos y errores. **El estado del nodo y el estado de su webhook son independientes.**

### Cadena completa, chunk y webhook: tres tamaños distintos

Un **chunk** es una pieza del protocolo incremental: puede declarar un objeto, publicar un escalar o cerrar un contenedor. Un **resultado** es una cadena terminada formada por muchos chunks. Un **webhook** es el mensaje HTTP que intenta entregar esa cadena al consumidor.

| Situación | Comportamiento | Qué puedes recuperar |
| --- | --- | --- |
| Un escalar, por ejemplo un string de 300 KiB, no cabe en un chunk de 256 KiB incluido su envelope | La ejecución falla con `RESULT_VALUE_TOO_LARGE`. El string no se divide automáticamente. La cadena incompleta no se publica como terminada. | Los resultados y chunks confirmados antes del fallo siguen accesibles hasta expirar. |
| Una cadena de 600 KiB está formada por propiedades que caben en sus chunks, pero su mensaje supera `webhooks.maxPayloadBytes` (256 KiB por defecto) | La cadena puede terminar correctamente. Sólo su entrega falla con `WEBHOOK_PAYLOAD_TOO_LARGE`, sin enviar HTTP ni una versión reducida. | Descarga toda la cadena desde `/results/{nodeId}`; su JSON no tiene el límite de tamaño del webhook. |
| Se agota un presupuesto de ejecución o publicación, como `executions.v2.maxPublishedBytes` (1 GiB por defecto) | La ejecución termina con `EXECUTION_BUDGET_EXCEEDED`. | Se conservan los resultados previamente confirmados; revisa también el fallo global. |
| El receptor está lento o caído | La entrega se reintenta cuando corresponde. Una cola llena retrasa notificaciones; el engine puede seguir produciendo dentro de sus presupuestos. | Usa la API; si vence el plazo de entrega, revisa el fallo en `/webhooks`. |

El límite de **chunk es fijo, 256 KiB**. El límite de **webhook es configurable** mediante `webhooks.maxPayloadBytes` y comprende datos más envelope. Aumentar ese valor no amplía el máximo del chunk. Consulta **Servicio y arranque** para ver los límites efectivos.

### Ejemplo: idioma, Bulbasaur y el siguiente Pokémon

V2 puede resolver `idioma` y varios Pokémon a la vez. Bulbasaur aparece cuando su llamada a `especie` y todos los demás descendientes terminan; no necesita esperar a los otros Pokémon. Si Charizard termina primero, aparece primero, conservando su índice original. Cada resultado queda fijo en el viewer y puede recibirse por webhook según la política elegida.

Si `especie` falla definitivamente, Bulbasaur puede publicarse como `completed_with_errors`, con datos válidos y diagnósticos. Un fallo sin datos puede producir `failed` con valor `null`. Una colección vacía produce cero elementos; una cancelación o fallo global no convierte una cadena interrumpida en completa. Usa el estado y el terminal para distinguir esos casos.

### Chunks, descarga y compatibilidad

`/chunks?after=0&limit=100` permite seguir el protocolo de bajo nivel; su cursor es independiente del de `/results`. Las ramas pueden intercalarse: usa `nodeId`, `parentId` e `index`, no un rango contiguo de chunks para reconstruir una cadena. `/tree` explora hijos por ramas. `/export` descarga un corte de chunks en NDJSON: comprueba `export_end` y `complete: true`; si `terminal` es `false`, es un snapshot en curso, no el resultado final.

Los resultados expirados devuelven `410`; los cursores inválidos o adelantados, `422`. Un owner distinto, ID inexistente o nodo todavía no publicado devuelve `404`. Usa el mismo owner y permiso de lectura; los links no son públicos. En el playground, las rutas equivalentes usan `/playground/api/v2/executions` y su sesión.

Esta entrega por cadenas aplica a ejecuciones nuevas con `v2.resultsEnabled: true`. Las anteriores a la migración 000003 mantienen chunks, árbol y webhook terminal; `/results` devuelve `409 RESULTS_UNAVAILABLE`. No se generan eventos retrospectivos.

La retención de resultados es configurable (7 días desde el cierre por defecto). La ruta `/v2` selecciona el comportamiento; `definition.version` continúa siendo `1`.
