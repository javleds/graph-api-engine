## V1 · El grafo en una respuesta final

V1 ensambla el resultado completo antes de publicarlo. Mientras la ejecución está en curso puedes consultar su estado y progreso, pero todavía no descargar sus datos. Usa esta versión cuando tu consumidor quiera procesar una respuesta final con todas las raíces.

### Cómo consultar los resultados

1. Envía el request a `POST /v1/executions`. La respuesta `202` contiene `id` y links de seguimiento y resultado.
2. Consulta `GET /v1/executions/{id}` mientras el estado sea `pending` o `running`.
3. Si termina en `completed` o `completed_with_errors`, consulta `GET /v1/executions/{id}/result`. La propiedad `result` contiene el grafo; `errors`, `errorSummary` y `roots` permiten interpretar resultados parciales.
4. Si termina en `failed`, consulta el diagnóstico en el estado. El endpoint de resultado responde `409 EXECUTION_FAILED`.

```http
GET /v1/executions/{id}/result
Authorization: Bearer TOKEN_DEL_ENGINE
```

No hay cursor de resultados ni endpoints de chunks en V1. Consultar `/result` antes de tiempo devuelve `409 RESULT_NOT_READY`. Un resultado expirado devuelve `410 RESULT_EXPIRED`.

### Si incluyes webhook en el request

El webhook es opcional. Si lo omites o envías `null`, consulta por API; no se enviarán notificaciones. Si lo incluyes, se genera un evento al terminar la ejecución, también cuando termina con errores. No se envía un evento por cada Pokémon.

Este fragmento corresponde al campo `webhook` del request:

```json
{
  "url": "https://consumer.example/events",
  "payload": "result"
}
```

| Configuración | Qué recibe tu endpoint |
| --- | --- |
| `payload: "status"`, u omitido | Estado terminal, diagnósticos y links. No incluye los datos del grafo. |
| `payload: "result"` | Incluye el grafo final si la ejecución no falló y el mensaje cabe en el límite. Comprueba `resultIncluded`. |

El tipo del evento es `execution.completed`, `execution.completed_with_errors` o `execution.failed`. Un fallo de entrega no cambia el estado del procesamiento. Los reintentos pueden duplicar la notificación: deduplica por `eventId`. El mismo evento conserva su identidad y contenido entre intentos.

### Límites y qué hacer cuando se superan

| Situación | Comportamiento | Acción del consumidor |
| --- | --- | --- |
| El resultado completo supera `executions.maxResultBytes` (16 MiB por defecto) | La ejecución falla con `RESULT_TOO_LARGE`; no hay resultado final descargable. | Reduce la consulta o usa V2 para consumir cadenas progresivamente. |
| El webhook con `payload: "result"` supera `webhooks.maxPayloadBytes` (256 KiB por defecto, incluido el envelope) | Se omite el grafo y se envía el aviso con `resultIncluded: false` y `resultOmittedReason: "size_limit"`. También pueden omitirse detalles de errores para respetar el límite. | Recupera el grafo desde `/v1/executions/{id}/result` mientras siga disponible. |
| El receptor no responde o rechaza la notificación | Se aplican los reintentos elegibles hasta agotar intentos o plazo. No se garantiza la entrega. | Consulta el estado y recupera el resultado por API. |

Por ejemplo, un grafo de **600 KiB** puede caber en el límite de resultado V1 de 16 MiB y no caber en un webhook de 256 KiB. La ejecución puede terminar correctamente y el webhook llegar sin sus datos; la API sigue siendo la fuente para descargarlos.

### Ejemplo: idioma y Pokémon con especie

Con una raíz `idioma` y una colección `pokemon`, cada elemento enriquecido con `especie`, V1 espera al cierre de toda la ejecución. Entonces `/result` entrega un objeto con ambas raíces. Aunque `idioma` o Bulbasaur terminen antes, no se publican individualmente por la API de resultados.

Si un hijo falla y quedan datos válidos, el resultado puede ser `completed_with_errors`: revisa los diagnósticos y no trates un valor `null` como prueba de éxito.

### Acceso y conservación

Usa un token del mismo propietario de la ejecución con permiso de lectura; un enlace no otorga acceso anónimo. En el playground las rutas equivalentes usan `/playground/api/executions` y su sesión. La retención de resultados es configurable (7 días desde el cierre por defecto); los metadatos pueden durar más.

La versión de la API se selecciona mediante `/v1`, no cambiando `definition.version`, que sigue siendo `1`. Los valores de tamaño indicados son defaults: consulta **Servicio y arranque** para ver la configuración efectiva.
