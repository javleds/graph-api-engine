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

### Orígenes abiertos en desarrollo local

Para probar APIs y webhooks públicos sin registrar cada dominio, configurar ambos campos en `security.egress`:

```json
"allowHTTPForDevelopment": true,
"allowAnyOriginForDevelopment": true
```

`allowAnyOriginForDevelopment` es `false` por defecto y exige el modo de desarrollo existente (`allowHTTPForDevelopment`). Cuando está activo, las listas de origins dejan de restringir destinos y pueden estar vacías; la validación del playground y los transportes usan la misma política. En producción conservar ambos flags en `false` y configurar las allowlists.

Esta opción abre orígenes, no la red privada: localhost, redes privadas y destinos de metadata mantienen sus controles. Para un proveedor/webhook local, configurar también el CIDR concreto en `allowedPrivateCIDRs`. Reiniciar el servicio tras cambiar la configuración.

## Uso HTTP

Consulta la [guía para construir el JSON](docs/api-request-guide.md), el [schema aceptado](api_request_schema.json) y el [ejemplo completo de varias ramas](contracts/examples/commerce_graph.json).

Enviar `POST /v1/executions` con bearer del engine, `Content-Type: application/json` y una definición como la de `contracts/examples/pokemon_full.json`. La respuesta `202` entrega el UUID y los links de seguimiento y resultado.

`Idempotency-Key` permite recuperar la misma ejecución con el mismo contenido normalizado; contenido diferente produce `409`. Las keys duran mientras se conserve la metadata. Después de su expiración, reutilizar una key puede crear otra ejecución.

Consultar `GET /v1/executions/{id}` y `GET /v1/executions/{id}/result` con el mismo owner. El resultado terminal usa `result`, `errors`, `errorSummary` y `roots`; `completed_with_errors` conserva datos válidos y posiciones de nodos fallidos. Un resultado no listo o una ejecución fallida responde `409`; uno expirado responde `410`. Una colección vacía válida cuenta como éxito.

Las cuotas se comparten por proveedor/credencial y endpoint. Un arranque frío espera la ventana conservadora definida en §14 antes de enviar. Configurar grupos administrados cuando varios tokens u origins compartan una cuota real. Caché y flights evitan intentos físicos sin eliminar relaciones de salida.

## Playground local

Abrir `/playground` en el mismo servicio (por ejemplo, `http://127.0.0.1:8080/playground`). Pegar el request completo, incluidas las credenciales del proveedor si se requieren, y pulsar **Ejecutar request**. La página sigue el estado y muestra el resultado al terminar; el modo árbol permite expandir objetos/arrays y el modo raw conserva el texto recibido, de solo lectura y copiable.

El botón **Validar JSON** comprueba sintaxis, schema y referencias sin crear un job ni consultar al proveedor. Indica si el input cumple el contrato y muestra el campo o la ubicación del error cuando están disponibles. Los números de línea del editor no son seleccionables; al editar se descarta el diagnóstico anterior.

La depuración muestra el request enviado y las respuestas HTTP de creación, estado y resultado. Detener el seguimiento no cancela el job; se puede retomar mediante su ID. No hay historial ni credenciales guardadas en el navegador. El ejemplo de la página usa un proveedor ilustrativo: adaptar URL, selector y credenciales a un destino permitido por la configuración.

El playground está **abierto para uso local** y sus endpoints `/playground/api/executions` usan el owner reservado `system:playground`, compartido entre visitantes. Quien pueda acceder puede crear jobs y consultar los jobs de esa identidad si conoce su ID. Mantener el servicio en la red local prevista. `/v1/executions` sigue autenticado y no permite leer estos jobs con otro owner. Las cuotas, límites, validación y egress siguen aplicando. No hay tokens automáticos expuestos ni autenticación configurable por `.env` en esta versión.

Los webhooks mantienen su formato y links `/v1`; estos no dan acceso anónimo al resultado. Para una ejecución del playground, consultar su ID desde la página o `/playground/api/executions/{id}`. Los assets se incluyen en el binario y contenedor, sin CDN ni proceso frontend adicional. Navegadores sin soporte para conservar los literales numéricos usan raw para evitar redondear IDs grandes.

Pruebas de navegador (Node.js 22 o posterior, solo desarrollo):

```sh
npm ci
npx playwright install chromium
make playground-test
```

La suite inicia el servicio y un proveedor controlado en loopback con SQLite y claves temporales. No usa APIs ni secretos reales. Véase [la especificación del playground](docs/specs/002-playground-local.md).

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

Compose monta los tres archivos `secrets/engine-identity-key`, `secrets/engine-encryption-key` y `secrets/engine-principals.json` individualmente. Mantener el directorio host `secrets` con permisos `0700` y esos archivos con lectura para el UID del contenedor (`0644` dentro del directorio protegido es suficiente). No se monta un token Bearer en texto plano. Si se cambian nombres o se agregan claves para rotación, actualizar los mounts junto con la configuración.

Los bind mounts usan `create_host_path: false`: si falta un archivo, Compose falla de inmediato en vez de crear un directorio con nombre `.json`. Si un arranque anterior creó `config/config.json` como directorio, detener el servicio, conservar cualquier contenido existente y reemplazar el directorio vacío por el archivo de configuración antes de recrearlo.

El runtime usa UID/GID `65532`, certificados CA y filesystem de solo lectura. `/data` pertenece a ese UID y contiene `engine.db` y `cache.db`; `/tmp` es temporal. Preparar permisos de lectura de configuración y secretos para ese UID. El volumen nuevo hereda el ownership de `/data`; para volúmenes existentes, corregirlo antes del arranque.

Compose publica únicamente `127.0.0.1:8080`. Para exposición remota, agregar terminación TLS administrada y reglas de red. El servicio necesita salida a los origins autorizados; no depende de Redis ni de un broker. Eliminar el volumen elimina ejecuciones, resultados, notificaciones y caché. La política `restart` no reanuda árboles interrumpidos.

La [matriz de pruebas](docs/testing.md) relaciona los casos de aceptación del spec con sus verificaciones.
