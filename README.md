# API Execution Engine

Servicio Go asíncrono para obtener y enriquecer árboles de datos HTTP. Una instancia combina API, worker, caché SQLite, cuotas compartidas y webhooks durables.

El contrato normativo está en [el spec v1.1](docs/specs/spec-api-execution-engine-go-v1.1.md). El [plan incremental](docs/specs/001-plan-api-execution-engine-go-v1.1.md) describe los commits; las capacidades futuras de Redis, reanudación de nodos y múltiples réplicas quedan fuera del MVP.

## Desarrollo

Requiere Go 1.27.1 y, para tests de carreras, las herramientas C del sistema. Las dependencias están fijadas en `go.mod` y `go.sum`. El binario de producción no necesita CGO.

```sh
make check
make coverage fuzz
make build
make vuln
./bin/api-engine validate --file contracts/examples/pokemon_full.json
./bin/api-engine version
```

Los tests usan SQLite temporal y proveedores locales controlados; no requieren credenciales ni APIs reales. `validate` comprueba el contrato sin abrir bases ni conectar a proveedores. Con `--config` también aplica las restricciones del operador.

## Configuración y secretos

Copiar `config/config.example.json` a `config/config.json` y ajustar rutas locales, URL pública y allowlists de proveedores y webhooks. El ejemplo contiene valores ilustrativos y no es una configuración lista para producción. Consultar §21 del spec para todos los campos y defaults.

Los archivos de identidad y cifrado contienen **32 bytes binarios aleatorios**, cada uno generado independientemente. Guardarlos fuera del repositorio, accesibles solo al operador y al UID del runtime. `activeKeyId` identifica la clave de cifrado vigente; conservar las claves anteriores para leer registros durante rotación. La clave de identidad debe permanecer estable para mantener scopes e idempotencia.

El archivo de principals relaciona `ownerId`, SHA-256 hexadecimal del token bearer, `enabled` y permisos (`executions:create`, `executions:read`, `metrics:read`). Generar tokens de alta entropía y entregarlos fuera de banda. El formato exacto está en §21.5.

HTTPS es obligatorio salvo desarrollo explícito. Para pruebas locales, habilitar `allowHTTPForDevelopment`, declarar el origin exacto y autorizar el CIDR privado concreto. La autenticación solo puede desactivarse escuchando en loopback. No usar la desactivación de autenticación en el contenedor expuesto.

```sh
./bin/api-engine migrate --config config/config.json
./bin/api-engine serve --config config/config.json
```

`serve` valida configuración y claves, adquiere el lock de instancia, migra y reconcilia el estado antes de aceptar tráfico. No ejecutar varios procesos sobre el mismo directorio. `ENGINE_CONFIG`, `ENGINE_HTTP_ADDRESS`, `ENGINE_HTTP_PUBLIC_BASE_URL` y `ENGINE_LOG_LEVEL` son los overrides admitidos. `--config` prevalece sobre `ENGINE_CONFIG`.

## Uso HTTP

Enviar `POST /v1/executions` con bearer del engine, `Content-Type: application/json` y una definición como la de `contracts/examples/pokemon_full.json`. La respuesta `202` entrega el UUID y los links de seguimiento y resultado.

`Idempotency-Key` permite recuperar la misma ejecución con el mismo contenido normalizado; contenido diferente produce `409`. Las keys duran mientras se conserve la metadata. Después de su expiración, reutilizar una key puede crear otra ejecución.

Consultar `GET /v1/executions/{id}` y `GET /v1/executions/{id}/result` con el mismo owner. El resultado terminal usa `result`, `errors`, `errorSummary` y `roots`; `completed_with_errors` conserva datos válidos y posiciones de nodos fallidos. Un resultado no listo o una ejecución fallida responde `409`; uno expirado responde `410`. Una colección vacía válida cuenta como éxito.

Las cuotas se comparten por proveedor/credencial y endpoint. Un arranque frío espera la ventana conservadora definida en §14 antes de enviar. Configurar grupos administrados cuando varios tokens u origins compartan una cuota real. Caché y flights evitan intentos físicos sin eliminar relaciones de salida.

## Operación

`/health/live` indica vida del proceso y `/health/ready` verifica almacenamiento crítico y readiness. Las métricas requieren `metrics:read`; los logs JSON no incluyen bodies ni credenciales.

SIGTERM/SIGINT cierra admisión, detiene reclamación de webhooks, espera la gracia y cancela trabajo restante con una ventana independiente de persistencia. Tras una caída abrupta, los jobs pendientes/activos se marcan `PROCESS_INTERRUPTED`; no se reanuda el árbol. El consumidor decide si crea una nueva ejecución.

Los webhooks pueden duplicarse: deduplicar persistentemente por `eventId`. Para HMAC, verificar `eventId.timestamp.rawBody` con SHA-256, comparación constante y una tolerancia de tiempo. Cada intento conserva evento/payload y renueva timestamp/firma. El fallo de entrega no cambia el estado terminal del job.

La retención de metadata supera la de resultados y la ventana de entrega. Para backup sencillo, detener limpiamente la instancia y copiar el directorio de datos completo junto con un respaldo separado y protegido de las claves. No copiar únicamente `engine.db` mientras WAL está activo. Restaurar en un directorio local, con sus permisos y claves, antes de arrancar una sola instancia.

## Contenedor

```sh
docker compose config
docker compose build
docker compose up -d api-engine
docker compose logs -f api-engine
```

El runtime usa UID/GID `65532`, certificados CA y filesystem de solo lectura. `/data` pertenece a ese UID y contiene `engine.db` y `cache.db`; `/tmp` es temporal. Preparar permisos de lectura de configuración y secretos para ese UID. El volumen nuevo hereda el ownership de `/data`; para volúmenes existentes, corregirlo antes del arranque.

Compose publica únicamente `127.0.0.1:8080`. Para exposición remota, agregar terminación TLS administrada y reglas de red. El servicio necesita salida a los origins autorizados; no depende de Redis ni de un broker. Eliminar el volumen elimina ejecuciones, resultados, notificaciones y caché. La política `restart` no reanuda árboles interrumpidos.

La [matriz de pruebas](docs/testing.md) relaciona los casos de aceptación del spec con sus verificaciones.
