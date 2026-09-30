# Configurar y arrancar el servicio

Desde la raíz del repositorio y después de compilar, ejecutar `./bin/api-engine init`. El comando crea los archivos locales faltantes: `.env` desde `.env.example`, `config/config.yml` desde `config/config.example.yml`, y las dos claves de 32 bytes más `secrets/engine-principals.json`. Imprime el bearer token del principal `local-developer` sólo al crear el archivo de principals; guardarlo en un gestor de secretos. Nunca lo guarda en texto plano y no sobrescribe archivos existentes, por lo que es seguro volver a ejecutarlo.

`APP_URL` define la URL pública y `APP_PORT` define el puerto del proceso y de Compose. El archivo contiene las credenciales del playground; no debe versionarse.

`config/config.yml` es la configuración de operador. Usa interpolación `${VARIABLE}` para consumir `.env`. Las listas `security.egress.upstreamAllowedOrigins` y `webhookAllowedOrigins` aceptan URLs exactas, `https://*.example.com` y `*`; HTTP requiere `allowHTTPForDevelopment: true` y las IP privadas siguen restringidas por CIDR.

La sección `ui` controla la consola web completa (login, documentación, ejemplos, playground y sus assets):

```yaml
ui:
  enabled: true       # false devuelve 404 en las rutas de la consola
  language: es        # es o en; se selecciona al iniciar el servicio
```

El idioma predeterminado es español. Las propiedades técnicas, valores booleanos, rutas y bloques de código no se traducen. La API, los workers, health checks y métricas continúan disponibles aunque `ui.enabled` sea `false`.

Para iniciar toda la aplicación:

```sh
docker compose up -d --build
curl http://localhost:8080/health/ready
```

Abrir `/login`, iniciar sesión y continuar hacia `/playground`. El proceso también puede iniciarse directamente con `./bin/api-engine serve --config config/config.yml`.

## Caché de respuestas upstream

La sección de operador `cache` define **capacidad y seguridad**, no activa caché para todos los requests. El consumidor decide por grafo si reutiliza respuestas con `definition.api.cache: { ttl: "5m" }` o por nodo con `calls[].cache`. Cada entrada conserva la respuesta HTTP base cifrada; el resultado enriquecido de una ejecución no se guarda como una entrada de caché.

`maxEntryBytes` rechaza sólo el almacenamiento de respuestas grandes: la ejecución todavía puede usar la respuesta si respeta los demás límites. `maxEntries` y `maxLogicalBytes` son presupuestos globales; al alcanzarlos se expulsan entradas antiguas. `maxTTL` es el techo que un consumidor puede pedir, no un TTL predeterminado. Para invalidar todas las respuestas almacenadas, un principal con `cache:clear` usa `POST /v1/cache/clear` o `POST /v2/cache/clear`; afecta a todos los consumidores de esta instancia.

## Cuotas compartidas y concurrencia

`fetch.maxConcurrentHTTPRequests` es el techo global de requests upstream simultáneos de la instancia. Es distinto de `api.limits`: el request declara la cuota conocida de un proveedor y el engine aplica pacing a cada intento físico, incluidos retries y paginación. Un `429` o `Retry-After` publica un cooldown para el scope afectado, por lo que las demás ejecuciones que lo comparten esperan; el cooldown se conserva en SQLite al reiniciar.

Sin un grupo administrado, las ejecuciones con el mismo origin, credencial upstream y `api.limits` comparten cuota. Valores distintos de `api.limits` tienen scopes independientes. Si el proveedor limita por cuenta, IP, varias API keys o varios origins, declarar un grupo del operador para que todos cumplan la misma restricción:

```yaml
throttle:
  quotaGroups:
    - name: proveedor-principal
      origins: ["https://api.proveedor.example"]
      credentialMode: shared
      limits:
        requests: 100
        per: "1m"
        concurrency: 4
```

Con `credentialMode: shared`, todas las credenciales usadas contra esos origins comparten la cuota. Con `separate` —el default— cada credencial tiene su cuota propia. Cuando un grupo cubre el origin, sus límites prevalecen sobre `api.limits` y se comparten entre las ejecuciones del grupo. `call.limits` sigue añadiendo restricciones del endpoint. Dos instancias con directorios de datos diferentes no comparten pacing; antes de escalar horizontalmente se necesita una coordinación distribuida.

Para invalidar la caché de forma controlada, conceder el permiso `cache:clear` a los principals de API seleccionados y usar las rutas versionadas documentadas. Las sesiones autenticadas del playground también pueden vaciar la caché compartida de respuestas tras una confirmación; consultar [Mantenimiento de caché](cache-maintenance.md).
