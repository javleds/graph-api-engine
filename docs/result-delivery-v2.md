## V2 · Cada cadena completa, sin esperar al resto

V2 publica resultados definitivos durante la ejecución. Una raíz `single` está lista al terminar ella y todos sus hijos; una raíz `collection` publica cada elemento con todos sus descendientes. La cadena ya publicada no cambia. El recorrido actual es secuencial: las raíces siguen el orden del request, y una llamada lenta puede retrasar lo que viene después.

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
  "payload": "status"
}
```

En V2, `payload` admite sólo `"status"` o puede omitirse; `"result"` se rechaza con HTTP `422`. Esto no impide recibir los datos: los eventos de nodo incluyen automáticamente la cadena en **`result`**, equivalente a **`data`** en la API del nodo. El aviso terminal contiene el resumen de la ejecución, no todo el grafo.

| Evento | Qué significa | Qué contiene |
| --- | --- | --- |
| `node.finished` | Esta cadena terminó. Su `status` puede ser `completed`, `completed_with_errors` o `failed`. | `eventId`, `executionId`, `nodeId`, `sequence`, estado, `result`, diagnósticos y links. |
| `execution.completed`, `execution.completed_with_errors` o `execution.failed` | El engine dejó de producir resultados. | Resumen, `resultCount`, `lastResultSequence`, `lastSequence` de chunks y links. No confirma que todos los webhooks de nodo hayan llegado. |

Puede haber duplicados y entregas fuera de orden. Deduplica por `eventId` y no interpretes el orden HTTP como el orden de la colección. Reintentos o latencia pueden hacer que el aviso terminal llegue antes que una entrega de nodo. Puedes recuperar lo faltante mediante `/results`.

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

Con `idioma` primero en el request, V2 publica esa raíz cuando termina. Después, Bulbasaur aparece cuando su llamada a `especie` y todos los demás descendientes terminan; no necesita esperar al siguiente Pokémon. Cada resultado queda fijo en el viewer y puede recibirse por webhook.

Si `especie` falla definitivamente, Bulbasaur puede publicarse como `completed_with_errors`, con datos válidos y diagnósticos. Un fallo sin datos puede producir `failed` con valor `null`. Una colección vacía produce cero elementos; una cancelación o fallo global no convierte una cadena interrumpida en completa. Usa el estado y el terminal para distinguir esos casos.

### Chunks, descarga y compatibilidad

`/chunks?after=0&limit=100` permite seguir el protocolo de bajo nivel; su cursor es independiente del de `/results`. `/tree` explora hijos por ramas. `/export` descarga un corte de chunks en NDJSON: comprueba `export_end` y `complete: true`; si `terminal` es `false`, es un snapshot en curso, no el resultado final.

Los resultados expirados devuelven `410`; los cursores inválidos o adelantados, `422`. Un owner distinto, ID inexistente o nodo todavía no publicado devuelve `404`. Usa el mismo owner y permiso de lectura; los links no son públicos. En el playground, las rutas equivalentes usan `/playground/api/v2/executions` y su sesión.

Esta entrega por cadenas aplica a ejecuciones nuevas con `v2.resultsEnabled: true`. Las anteriores a la migración 000003 mantienen chunks, árbol y webhook terminal; `/results` devuelve `409 RESULTS_UNAVAILABLE`. No se generan eventos retrospectivos.

La retención de resultados es configurable (7 días desde el cierre por defecto). La ruta `/v2` selecciona el comportamiento; `definition.version` continúa siendo `1`.
