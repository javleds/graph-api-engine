# Mantenimiento de caché

La caché de respuestas se puede vaciar bajo demanda mediante cualquiera de las versiones de API:

```text
POST /v1/cache/clear
POST /v2/cache/clear
```

Ambas rutas realizan la misma operación global sobre la caché SQLite de respuestas del servicio. Requieren un principal Bearer con el permiso dedicado `cache:clear` y no llevan body. Las sesiones autenticadas del playground pueden realizar la misma operación mediante `POST /playground/api/cache/clear`; la consola pide confirmación antes de enviarla.

```sh
curl --request POST "$ENGINE_URL/v1/cache/clear" \
  --header "Authorization: Bearer $ENGINE_TOKEN"
```

Una solicitud exitosa devuelve `200 OK`:

```json
{"deletedEntries":42}
```

`deletedEntries` es el número de filas eliminadas cuando se ejecutó el borrado; `0` es un no-op exitoso. Una credencial ausente o insuficiente devuelve `401` o `403`. Los fallos del almacenamiento de caché devuelven `503 STORAGE_UNAVAILABLE`.

El borrado afecta a todos los owners porque la caché es compartida por el proceso, aunque sus claves aíslan a cada owner. Sólo elimina respuestas upstream ya almacenadas. No cancela ejecuciones, no borra resultados ni metadata de ejecuciones, no altera cuotas ni cooldowns, no cierra conexiones HTTP y no detiene requests que ya están en vuelo. Una request que termina concurrentemente puede volver a llenar la caché después del borrado.
