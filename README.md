# API Execution Engine

Servicio Go asíncrono para obtener y enriquecer árboles de datos HTTP. Una instancia combina API, worker, caché SQLite, cuotas compartidas y webhooks durables.

El contrato normativo está en [el spec v1.1](docs/specs/spec-api-execution-engine-go-v1.1.md). El [plan incremental](docs/specs/001-plan-api-execution-engine-go-v1.1.md) describe los commits; Redis y múltiples réplicas siguen fuera de alcance. La [v2](docs/specs/005-api-execution-engine-v2.md) añade resultados por fragmentos y recuperación durable del árbol.

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

Después de compilar, ejecutar `./bin/api-engine init` desde la raíz del repositorio. Crea `secrets/` con permisos restringidos, copia `.env.example` a `.env` y `config/config.example.yml` a `config/config.yml`, y genera las claves y un principal local. Guarda el bearer token que imprime el comando: se muestra sólo en la primera creación y no se guarda en texto plano. El inicializador no sobrescribe archivos existentes; revisar y ajustar `.env` y `config/config.yml` antes de iniciar el servicio.

El ejemplo contiene valores ilustrativos y no es una configuración lista para producción. Consultar §21 del spec para todos los campos y defaults.

Los archivos de identidad y cifrado contienen **32 bytes binarios aleatorios**, cada uno generado independientemente. Guardarlos fuera del repositorio, accesibles solo al operador y al UID del runtime. `activeKeyId` identifica la clave de cifrado vigente; conservar las claves anteriores para leer registros durante rotación. La clave de identidad debe permanecer estable para mantener scopes e idempotencia.

El archivo de principals relaciona `ownerId`, SHA-256 hexadecimal del token bearer, `enabled` y permisos (`executions:create`, `executions:read`, `metrics:read`, `cache:clear`). Generar tokens de alta entropía y entregarlos fuera de banda. El formato exacto está en §21.5.

HTTPS es obligatorio salvo desarrollo explícito. Para pruebas locales, habilitar `allowHTTPForDevelopment`, declarar el origin exacto y autorizar el CIDR privado concreto. La autenticación solo puede desactivarse escuchando en loopback. No usar la desactivación de autenticación en el contenedor expuesto.

```sh
./bin/api-engine migrate --config config/config.yml
./bin/api-engine serve --config config/config.yml
```

`serve` valida configuración y claves, adquiere el lock de instancia, migra y reconcilia el estado antes de aceptar tráfico. No ejecutar varios procesos sobre el mismo directorio. `ENGINE_CONFIG`, `ENGINE_HTTP_ADDRESS`, `ENGINE_HTTP_PUBLIC_BASE_URL` y `ENGINE_LOG_LEVEL` son los overrides admitidos. `--config` prevalece sobre `ENGINE_CONFIG`.

La configuración del operador acepta exclusivamente YAML (`.yml` o `.yaml`). Sus valores escalares pueden incluir ${VARIABLE}; la variable debe existir y no estar vacía. Un valor que sea solo ${VARIABLE} conserva tipos YAML como número o booleano; una referencia dentro de texto produce un string. El loader rechaza aliases, merges, claves duplicadas, documentos múltiples, valores `null` y campos desconocidos; decodifica directamente en estructuras Go tipadas con defaults y validación declarativa. La carga local de `.env` no sobrescribe el entorno del proceso.

### Orígenes abiertos en desarrollo local

Para probar APIs y webhooks públicos sin registrar cada dominio, configurar ambos campos en `security.egress`:

```yaml
allowHTTPForDevelopment: true
allowAnyOriginForDevelopment: true
```

`allowAnyOriginForDevelopment` es `false` por defecto y exige el modo de desarrollo existente (`allowHTTPForDevelopment`). Cuando está activo, las listas de origins dejan de restringir destinos y pueden estar vacías; la validación del playground y los transportes usan la misma política. En producción conservar ambos flags en `false` y configurar las allowlists.

Esta opción abre orígenes, no la red privada: localhost, redes privadas y destinos de metadata mantienen sus controles. Para un proveedor/webhook local, configurar también el CIDR concreto en `allowedPrivateCIDRs`. Reiniciar el servicio tras cambiar la configuración.

## Uso HTTP

Consulta la [guía para construir el JSON](docs/api-request-guide.md), el [schema aceptado](api_request_schema.json) y el [ejemplo completo de varias ramas](contracts/examples/commerce_graph.json).

Enviar `POST /v1/executions` con bearer del engine, `Content-Type: application/json` y una definición como la de `contracts/examples/pokemon_full.json`. La respuesta `202` entrega el UUID y los links de seguimiento y resultado.

`Idempotency-Key` permite recuperar la misma ejecución con el mismo contenido normalizado; contenido diferente produce `409`. Las keys duran mientras se conserve la metadata. Después de su expiración, reutilizar una key puede crear otra ejecución.

Consultar `GET /v1/executions/{id}` y `GET /v1/executions/{id}/result` con el mismo owner. El resultado terminal usa `result`, `errors`, `errorSummary` y `roots`; `completed_with_errors` conserva datos válidos y posiciones de nodos fallidos. Un resultado no listo o una ejecución fallida responde `409`; uno expirado responde `410`. Una colección vacía válida cuenta como éxito.

Las cuotas se comparten por proveedor/credencial y endpoint. Un arranque frío espera la ventana conservadora definida en §14 antes de enviar. Configurar grupos administrados cuando varios tokens u origins compartan una cuota real. Caché y flights evitan intentos físicos sin eliminar relaciones de salida.

Un principal con el permiso dedicado `cache:clear` puede vaciar la caché compartida con `POST /v1/cache/clear` o `POST /v2/cache/clear`; las sesiones autenticadas del playground disponen del control equivalente con confirmación. Consultar [mantenimiento de caché](docs/cache-maintenance.md) para su efecto y respuesta.

Para resultados progresivos usar `/v2/executions`: [guía y protocolo](docs/api-v2-guide.md), [schema v2](contracts/create_execution_v2.schema.json) y [operación, recursos y recuperación](docs/v2-operations.md). El lenguaje `definition.version` continúa en 1. Las nuevas ejecuciones publican cada raíz single o elemento de colección cuando termina toda su cadena: `/results` lista eventos por cursor y `/results/{nodeId}` entrega el JSON completo. Si hay webhook, reciben automáticamente cada `node.finished` con sus datos y el aviso terminal; superar `webhooks.maxPayloadBytes` falla sólo esa entrega, sin truncar el resultado. Se conservan chunks, árbol y exportación NDJSON.

## Consola web

Copiar `.env.example` a `.env`, definir las credenciales del playground y arrancar con `docker compose up -d --build`. Abrir `/login`; la sesión firmada dura ocho horas y da acceso a `/playground`, `/docs` y `/examples`. La guía de operación está en [docs/service-configuration.md](docs/service-configuration.md).

En `/playground`, pegar el request completo, incluidas las credenciales del proveedor si se requieren, y pulsar **Ejecutar request**. Elegir v1 para resultado final o v2 para cadenas completas progresivas. V2 muestra páginas estables de resultados: las llegadas nuevas no reemplazan filas, selección ni detalle. Los resultados grandes se exploran por ramas o se descargan; chunks y NDJSON siguen disponibles en diagnóstico. El seguimiento puede continuar tras el cierre mientras quedan notificaciones pendientes, mostrando su estado separado. Las ejecuciones anteriores a la migración 000003 conservan el visor previo. En v1, la página sigue el estado y muestra el resultado al terminar; el modo árbol permite expandir objetos/arrays y el modo raw conserva el texto recibido, de solo lectura y copiable.

El botón **Validar JSON** comprueba sintaxis, schema y referencias sin crear un job ni consultar al proveedor. Indica si el input cumple el contrato y muestra el campo o la ubicación del error cuando están disponibles. Los números de línea del editor no son seleccionables; al editar se descarta el diagnóstico anterior.

La depuración muestra el request enviado y las respuestas HTTP de creación, estado y resultado. **Detener seguimiento** sólo desconecta el navegador; se puede retomar mediante el ID. **Abortar ejecución** solicita a Go el cierre durable del árbol y cancela las llamadas activas del job, tanto en v1 como en v2. La UI muestra **Cancelada**; la API conserva `status: failed` con `error.code: EXECUTION_CANCELLED`. En v2 se conservan los fragmentos ya confirmados. No hay historial ni credenciales guardadas en el navegador. El ejemplo de la página usa un proveedor ilustrativo: adaptar URL, selector y credenciales a un destino permitido por la configuración.

Los endpoints del playground requieren la cookie de sesión emitida por login y usan el owner reservado `system:playground`. `/v1/executions` y `/v2/executions` siguen usando Bearer tokens y no permiten leer estos jobs con otro owner. Las cuotas, límites, validación y egress siguen aplicando; el token de sesión nunca se expone al JavaScript.

### Licencia de PrimeVue 5

El playground usa componentes básicos de PrimeVue 5, que requieren una clave PrimeUI válida, Community o Commercial. Configurar `VITE_PRIMEUI_LICENSE_KEY` en `.env` antes de compilar el frontend. Para Docker Compose, ejecutar `docker compose build api-engine` después de configurar la variable; para desarrollo local, ejecutar `npm run build:web` antes de compilar el binario Go. Vite incorpora la clave en los assets del navegador durante el build: cambiar `.env` o la variable del contenedor después no actualiza un binario ya compilado. Obtener una clave para el desarrollador o equipo según las [condiciones oficiales](https://primeui.dev/licenses/community). No guardar claves personales en Git ni confirmar assets generados con una clave personal.

En v1 los webhooks mantienen su formato y links `/v1`; estos no dan acceso anónimo al resultado. Para una ejecución del playground, consultar su ID desde la página o `/playground/api/executions/{id}`. Los assets se incluyen en el binario y contenedor, sin CDN ni proceso frontend adicional. Navegadores sin soporte para conservar los literales numéricos usan raw para evitar redondear IDs grandes.

Pruebas de navegador (Node.js 22 o posterior, solo desarrollo):

```sh
npm ci
npx playwright install chromium
make playground-test
```

La suite inicia el servicio y un proveedor controlado en loopback con SQLite y claves temporales. No usa APIs ni secretos reales. Véase [la especificación del playground](docs/specs/002-playground-local.md).

## Operación

`/health/live` indica vida del proceso y `/health/ready` verifica almacenamiento crítico y readiness. Las métricas requieren `metrics:read`; los logs JSON no incluyen bodies ni credenciales.

SIGTERM/SIGINT cierra admisión, detiene reclamación de webhooks, espera la gracia y cancela trabajo restante con una ventana independiente de persistencia. Tras una caída abrupta, los jobs v1 pendientes/activos se marcan `PROCESS_INTERRUPTED`. Los jobs v2 reanudan el último checkpoint confirmado con el mismo ID y deadline, sin volver a consultar páginas confirmadas. Una petición en vuelo puede repetirse. Consultar [operación v2](docs/v2-operations.md) antes de migrar un volumen existente.

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

Los bind mounts usan `create_host_path: false`: si falta un archivo, Compose falla de inmediato en vez de crear un directorio con nombre `.yml`. Si un arranque anterior creó `config/config.yml` como directorio, detener el servicio, conservar cualquier contenido existente y reemplazar el directorio vacío por el archivo de configuración antes de recrearlo.

El runtime usa UID/GID `65532`, certificados CA y filesystem de solo lectura. `/data` pertenece a ese UID y contiene `engine.db` y `cache.db`; `/tmp` es temporal. Preparar permisos de lectura de configuración y secretos para ese UID. El volumen nuevo hereda el ownership de `/data`; para volúmenes existentes, corregirlo antes del arranque.

Compose publica únicamente `127.0.0.1:8080`. Para exposición remota, agregar terminación TLS administrada y reglas de red. El servicio necesita salida a los origins autorizados; no depende de Redis ni de un broker. Eliminar el volumen elimina ejecuciones, resultados, notificaciones y caché. La política `restart` no reanuda árboles interrumpidos.

La [matriz de pruebas](docs/testing.md) relaciona los casos de aceptación del spec con sus verificaciones.

### Cancelar mediante API

Enviar `POST /v1/executions/{id}/cancel` o `POST /v2/executions/{id}/cancel`, sin body, con token del mismo owner y permisos `executions:create` y `executions:read`. Devuelve 200 con el estado terminal confirmado; repetirlo no duplica la finalización ni el webhook. Si el job ya terminó, conserva ese resultado. El cierre usa la persistencia y webhook existentes; cancelar no revierte acciones ya realizadas en el proveedor. Una llamada compartida puede continuar para otros jobs. Ver [spec de cancelación](docs/specs/006-cancel-execution.md).
