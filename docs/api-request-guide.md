# Cómo construir el JSON de una ejecución

Esta guía describe el body de `POST /v1/executions` de la implementación v1. Aunque el spec se llama v1.1, **`definition.version` sigue siendo `1`**.

- [api_request_schema.json](../api_request_schema.json): JSON Schema Draft 2020-12 completo y autocontenido para editores y validadores.
- [Ejemplo completo de comercio](../contracts/examples/commerce_graph.json): varias raíces, ramas, ancestros, IDs escalares, paginación, caché y webhook.
- [Spec normativo](specs/spec-api-execution-engine-go-v1.1.md): contratos detallados, límites y catálogo de errores.

El schema publicado es una copia exacta de `contracts/create_execution_v1.schema.json` y del schema embebido en la app. Las pruebas comprueban su igualdad. Al modificar el contrato, actualizar los tres archivos juntos. El schema valida estructura; la app también valida referencias, políticas y restricciones del operador.

## 1. Elegir la forma del árbol

La definición representa **un árbol o un bosque de árboles**, con un único proveedor y autenticación común por ejecución. Cada nodo hace un GET, selecciona datos de la respuesta y ejecuta sus hijos para enriquecer esos datos.

| Relación que necesitas | Cómo representarla |
|---|---|
| Un recurso | Nodo `single`. |
| Una lista | Nodo `collection`; exige un array. |
| Uno a uno | Hijo `single` que recibe el ID del padre. |
| Uno a muchos | Hijo `collection` por cada instancia del padre. |
| Cadena de dependencias | Anidar `children`; usar `parent` o un ancestro por nombre. |
| Varias relaciones del mismo objeto | Varios nodos dentro de `children`. |
| Consultas independientes | Array en `definition.calls`; cada elemento es una raíz. |
| Muchos a muchos | Consultar la lista de relaciones/IDs y anidar un hijo que obtenga cada detalle. Cada rama conserva su relación. |

No hay aristas por ID entre nodos de la definición, joins entre hermanos o raíces, ciclos, recursión dinámica, condiciones, scripts, transformaciones arbitrarias ni paginación por cursor. Una relación compartida se expresa repitiendo la consulta en sus ramas; la caché o deduplicación de requests puede ahorrar HTTP sin eliminar elementos de salida. Para varios proveedores, crear ejecuciones separadas y componer sus resultados desde el cliente.

## 2. Request mínimo completo

Guardar este documento como `request.json`:

```json
{
  "definition": {
    "version": 1,
    "api": {
      "baseUrl": "https://example.com/api/v1",
      "limits": { "requests": 100, "per": "1m" }
    },
    "calls": {
      "name": "products",
      "type": "collection",
      "select": "data.items",
      "request": { "method": "GET", "path": "/products" }
    }
  }
}
```

Supone que `/api/v1/products` devuelve un objeto con un array en `data.items`. Si devuelve el array directamente, omitir `select`. Si devuelve un objeto individual, usar `single`.

```sh
make build
./bin/api-engine validate --file request.json
# Aplica también los límites y origins de tu configuración:
./bin/api-engine validate --file request.json --config config/config.json

curl --request POST "$ENGINE_URL/v1/executions" \
  --header "Authorization: Bearer $ENGINE_TOKEN" \
  --header 'Content-Type: application/json' \
  --header 'Idempotency-Key: catalog-import-001' \
  --data-binary @request.json
```

`ENGINE_TOKEN` autentica al cliente ante el engine. Las credenciales de `definition.api.auth` autentican al engine ante el proveedor; son independientes. `Idempotency-Key` es opcional y va en un header, nunca dentro del JSON. Reutilizarla con el mismo contenido normalizado recupera la ejecución; con otro contenido produce `409` mientras la key siga retenida.

La validación offline no hace requests ni garantiza que el proveedor devuelva los campos esperados. Los origins y destinos deben estar autorizados en el servicio que ejecutará el JSON.

## 3. Campos de la envoltura y del proveedor

| Campo | Obligatorio | Contenido |
|---|---|---|
| `definition` | Sí | Objeto con `version`, `api` y `calls`. |
| `webhook` | No | Configuración de notificación; omitir o usar `null` para desactivarla. |
| `definition.version` | Sí | Número `1`. |
| `definition.api` | Sí | Un proveedor común a todos los nodos. |
| `definition.calls` | Sí | Un nodo o un array no vacío de raíces con nombres únicos. |
| `api.baseUrl` | Sí | URL HTTPS absoluta; permite prefijo de path, sin query, fragmento ni credenciales en URL. |
| `api.limits` | Sí | `requests` y `per`; opcionalmente `concurrency`. |
| `api.auth` | No | Por defecto `none`; formatos en §8 de esta guía. |
| `api.headers` | No | Mapa de strings estáticos. |
| `api.cache` | No | `false` o `{ "ttl": "5m" }`; inicialmente deshabilitada. |

`baseUrl: https://example.com/api/v1` y `path: /products/10` producen `https://example.com/api/v1/products/10`. El slash inicial del nodo conserva el prefijo base. Solo se permite HTTP si el operador lo habilita explícitamente para desarrollo.

## 4. Definir un nodo y su request

| Campo del nodo | Obligatorio | Regla |
|---|---|---|
| `name` | Sí | Propiedad de salida; 1–64 caracteres, patrón `^[A-Za-z_][A-Za-z0-9_]*$`. `parent` está reservado. |
| `type` | Sí | `single` para un valor no-array; `collection` para un array. |
| `request` | Sí | Objeto con `method` y `path`. |
| `select` | No | Ruta por puntos; omitido o `null` selecciona el body completo. |
| `children` | No | Array de nodos recursivos, con nombres únicos entre hermanos. |
| `pagination` | No | Solo para `collection`; ver §7. |
| `limits` | No | Restricción adicional del endpoint; ver §9. |
| `cache` | No | Hereda la política del padre o de `api`; puede reemplazarla. |

`request.method` es obligatorio y solo acepta `"GET"`. `request.path` comienza con `/`, no con `//`; no admite una URL completa, query, fragmento, traversal ni backslashes. No existe un campo `body`.

`request.query` es un objeto cuyos valores son strings, números o booleanos. No acepta arrays, objetos ni `null`. Para una API que recibe una lista separada por comas, enviar un string como `"1,2,3"` si ese es su contrato; no hay parámetros query repetidos en v1.

```json
{
  "method": "GET",
  "path": "/companies/{parent.id}/users",
  "query": { "active": true, "perPage": 50, "region": "{parent.region}" },
  "headers": { "Accept-Language": "es" }
}
```

Este fragmento corresponde a `request` de un hijo cuyo padre contiene `id` y `region`. Los headers del nodo reemplazan los de `api.headers` por nombre, sin distinguir mayúsculas. No se heredan los headers de un nodo a sus hijos.

## 5. Selección, contextos y parámetros dinámicos

### Seleccionar la respuesta

| `select` | Significado |
|---|---|
| Omitido o `null` | Body JSON completo. |
| `data.items` | Propiedad `items` dentro de `data`. |
| `data.0.id` | `id` del primer elemento del array `data`. |

Ante un objeto, un segmento numérico busca una propiedad con ese nombre. Ante un array, busca un índice base cero. No hay JSONPath: no usar `$`, `*`, `[]`, filtros, funciones o segmentos vacíos. Las keys con un punto literal no son accesibles mediante este selector. Un campo ausente causa `SELECT_PATH_NOT_FOUND`; un campo existente con `null` conserva ese valor, sujeto al tipo del nodo.

`select` se evalúa antes de paginar y enriquecer. No es un mapa de campos ni cambia nombres de propiedades.

### Leer ancestros

Las sustituciones se admiten **solo en `request.path` y valores string de `request.query`**:

| Referencia | Lee |
|---|---|
| `{parent.id}` | `id` de la instancia inmediata que originó el hijo. |
| `{parent.value}` | Escalar normalizado del padre. |
| `{companies.id}` | `id` de la instancia del ancestro llamado `companies` en esta rama. |
| `{parent.address.country}` | Una propiedad anidada del padre. |

En una colección, `parent` es cada elemento seleccionado, no el array completo ni el body original. Los contextos se congelan antes del enriquecimiento: un hijo no puede leer lo agregado por otro hijo. Las raíces no tienen `parent`. No se puede leer la propia llamada, un hermano ni otra raíz.

Nombres repetidos en ramas distintas son válidos. Si dos ancestros de la misma rama comparten nombre, una referencia nominal a ese nombre es ambigua y se rechaza; `parent` sigue identificando al inmediato.

Los valores resueltos deben ser strings, números o booleanos. Una propiedad ausente produce `BINDING_PATH_NOT_FOUND`; un objeto, array o `null` no sirve como parámetro. El engine escapa el path y la query por separado: pasar datos sin preescapar. Los números conservan precisión; si tu cliente JavaScript ya redondeó un ID grande, el engine no puede recuperarlo: construir esos IDs como strings cuando sea necesario.

### Forma del resultado enriquecido

- Un objeto se copia y recibe una propiedad por cada hijo, usando el `name` de ese hijo.
- Un escalar con hijos se convierte en `{ "value": <escalar> }` y luego se enriquece.
- Sin hijos, los valores se conservan sin agregar `value`.
- `null` y arrays anidados no se pueden enriquecer: se registra `PARENT_NOT_ENRICHABLE`, se conserva la posición como `null` y se omiten sus hijos.
- Si un hijo se llama igual que un campo existente del objeto, la instancia falla con `OUTPUT_FIELD_COLLISION`. Elegir otro `name`; no hay sobrescritura.
- Las colecciones conservan posiciones y orden, aunque las ramas terminen en distinto momento.

Ejemplo: un nodo `productIds` recibe `[10, 20]` y tiene un hijo `details` que consulta `/products/{parent.value}`. Su valor enriquecido será:

```json
[
  { "value": 10, "details": { "id": 10, "title": "A" } },
  { "value": 20, "details": { "id": 20, "title": "B" } }
]
```

## 6. Ejemplo completo de varias ramas

Usar [commerce_graph.json](../contracts/examples/commerce_graph.json) como request completo. Los dominios y secretos son ilustrativos; reemplazarlos antes de enviarlo.

```text
companies[]                         /companies → data.items
├── profile                         /companies/{parent.id}/profile → data
└── users[]                         /companies/{parent.id}/users → data.items
    └── orders[]                    /companies/{companies.id}/users/{parent.id}/orders → data.items
        └── productIds[]            /orders/{parent.id}/product-ids → array de IDs
            └── details             /products/{parent.value} → data
catalog[]                           /catalog → data.items (raíz independiente)
```

Cada empresa debe incluir `id` y `region`; cada usuario y pedido debe incluir `id`. `orders` usa el ID del usuario inmediato y el ID/región de la empresa ancestro. `productIds` devuelve escalares que se normalizan para consultar `details`. Los objetos originales no deben incluir los nombres reservados para sus hijos (`profile`, `users`, `orders` o `productIds`, según el nivel).

Las dos raíces producen propiedades `companies` y `catalog` dentro de `result`. No dependen entre sí. Las páginas de empresas se recorren hasta el final; los usuarios se limitan a tres páginas por empresa. El perfil desactiva caché y el catálogo pide un TTL distinto. Todos los nodos siguen usando el mismo `api`.

```sh
./bin/api-engine validate --file contracts/examples/commerce_graph.json
```

## 7. Paginación por número de página

Fragmento de un nodo `collection`:

```json
{
  "name": "users",
  "type": "collection",
  "select": "data.items",
  "request": { "method": "GET", "path": "/users", "query": { "perPage": 50 } },
  "pagination": { "page": "page", "pageSize": "perPage", "start": 1, "pages": "all" }
}
```

| Campo | Regla / default |
|---|---|
| `page` | Obligatorio: nombre del parámetro query del número de página. |
| `pageSize` | Opcional: nombre del parámetro query que contiene el tamaño solicitado. |
| `start` | Entero ≥ 0; default `1`. |
| `pages` | `"all"` o entero positivo; default `"all"`. Es cantidad máxima de páginas, no número de página final. |

`page` no puede aparecer también en `request.query`. Si se declara `pageSize`, su valor en `query` debe ser un entero JSON positivo estático: `50`, no `"50"` ni `"{parent.size}"`. Ambos nombres deben ser distintos.

Se termina normalmente al recibir una página vacía, una página menor al tamaño esperado o al completar `pages`. Sin `pageSize`, la primera página no vacía fija el tamaño esperado. Sin `pagination`, solo se consulta una página.

Un error definitivo conserva las páginas ya aceptadas y detiene ese paginador. Una página mayor al tamaño esperado se descarta con `PAGINATION_SIZE_INCONSISTENT`. Un fallo de un hijo no detiene la paginación del padre. Cada instancia de padre tiene sus propios contadores.

`"all"` sigue sujeto a límites del servidor. No consume `next`, cursores, offsets, totales ni links de la respuesta. Si el proveedor devuelve páginas cortas antes del final, esta estrategia no garantiza recuperar todos sus datos.

## 8. Autenticación y headers

Usar una de estas formas en `api.auth`:

| Tipo | Objeto |
|---|---|
| Ninguna | `{ "type": "none" }` |
| Bearer | `{ "type": "bearer", "token": "token-del-proveedor" }` |
| Basic | `{ "type": "basic", "username": "usuario", "password": "clave" }` |
| Header | `{ "type": "header", "name": "X-API-Key", "value": "clave" }` |

Omitir `auth` equivale a `none`. No hay auth por nodo, OAuth refresh ni firma HMAC del proveedor. No declarar manualmente el header que controla `auth`: por ejemplo, `Authorization` junto con `bearer` o `basic`.

Los headers son estáticos. `${API_TOKEN}` es un string literal: el servidor no expande variables de entorno del JSON ni interpola ancestros en headers. Resolver las credenciales en el cliente antes de enviar.

No configurar `Host`, `Connection`, `Content-Length`, `Transfer-Encoding`, `TE`, `Trailer`, `Upgrade`, `Keep-Alive`, `Proxy-Authenticate`, `Proxy-Authorization` ni `User-Agent`. Tampoco repetir nombres con distinto casing dentro del mismo mapa. Los controles y saltos de línea son inválidos. El engine agrega `Accept: application/json` si falta. No sigue redirecciones del proveedor.

## 9. Cuotas, concurrencia y caché

```json
{ "requests": 100, "per": "1m", "concurrency": 4 }
```

`api.limits` exige `requests` entero positivo y `per`. `concurrency`, si se declara, es un entero positivo; si se omite, se usa el límite HTTP configurado por el operador. Las cuotas aplican a intentos físicos, incluidos retries, mediante pacing; no prometen ráfagas de 100 requests simultáneas.

`call.limits` puede contener `requests` + `per`, solo `concurrency`, o ambos. No puede estar vacío. Agrega restricciones del endpoint para todas sus páginas e IDs, sin sustituir la cuota global y sin limitar automáticamente a hijos que consultan otros endpoints. Las cuotas se comparten por proveedor/credencial; declaraciones incompatibles del mismo scope causan `QUOTA_POLICY_CONFLICT`.

Las duraciones usan un entero positivo y una sola unidad: `250ms`, `1s`, `5m`, `2h`, `7d`. No usar `0s`, `1.5s`, `1h30m`, negativos ni espacios; `1d` equivale a 24 horas.

| `cache` | Efecto |
|---|---|
| Omitida en `api` | Deshabilitada. |
| Omitida en un nodo | Hereda del padre, o de `api` si es raíz. |
| `false` | No lee ni escribe caché para ese nodo; sus descendientes heredan esta política salvo override. |
| `{ "ttl": "5m" }` | Habilita caché con esa exigencia de frescura. |

`true`, `null` y `{}` no son políticas válidas. Se cachea la respuesta HTTP base, no el árbol enriquecido; cada hijo se evalúa con su propia política. Headers del proveedor y límites operativos pueden impedir almacenar/reutilizar una respuesta. La deduplicación de requests simultáneas puede seguir activa aunque `cache` sea `false`.

## 10. Webhook opcional

Agregar junto a `definition`:

```json
{
  "url": "https://consumer.example.com/hooks/executions",
  "method": "POST",
  "headers": { "X-Integration": "catalog" },
  "auth": { "type": "hmac", "secret": "replace-with-webhook-secret" },
  "payload": "result"
}
```

Este fragmento es el valor de `webhook`, no un request completo. `url` es obligatorio y debe estar autorizado. `method` acepta `POST`, `PUT` o `PATCH` y por defecto es `POST`. `payload` acepta `status` (default) o `result`.

`auth` admite los cuatro formatos del proveedor y además `hmac`. No configurar los headers reservados `Content-Type`, `X-Engine-Event-Id`, `X-Engine-Timestamp` o `X-Engine-Signature`. HMAC firma `eventId.timestamp.rawBody` con SHA-256; ver [spec §19](specs/spec-api-execution-engine-go-v1.1.md#section-19) para el contrato de entrega.

La notificación puede representar éxito, éxito parcial o fallo y puede duplicarse por reintentos: deduplicar por `eventId`. `payload: result` está sujeto al límite de payload; el resultado puede omitirse explícitamente y seguir disponible en el endpoint de resultado. Un fallo del webhook no cambia el estado de la ejecución.

## 11. Validación y límites que el schema no expresa

El body debe ser JSON estricto: sin comentarios, comas finales, keys duplicadas, propiedades desconocidas o varios documentos concatenados. No enviar el archivo de configuración operativa como body de ejecución.

Los defaults operativos incluyen profundidad de definición 8, 128 nodos declarados, 16 hijos por nodo, 16 raíces, request de 262144 bytes y TTL máximo de 30 días. El operador puede cambiarlos; consultar su configuración. El schema permite estructuralmente hasta 128 raíces, pero eso no reemplaza el límite operativo (16 por defecto).

La app verifica además las referencias a ancestros, nombres únicos, seguridad de URL/headers, tamaño estático de página, cuotas y límites de duraciones. Durante la ejecución valida los tipos/datos reales, DNS/destino, límites de páginas, intentos, tiempo y tamaño de resultado. Pasar el schema no garantiza aceptación o éxito de una ejecución.

Para asociar el schema en VS Code, configurar el editor, **sin agregar `$schema` al body** (sería una propiedad desconocida):

```json
{
  "json.schemas": [
    { "fileMatch": ["/requests/*.json"], "url": "./api_request_schema.json" }
  ]
}
```

Guardar esa configuración en `.vscode/settings.json` y los requests en `requests/`. El archivo de schema contiene todos sus `$defs`; un validador Draft 2020-12 puede cargarlo sin resolver otros schemas del repositorio. La validación semántica completa offline se hace con `api-engine validate`.

## 12. Interpretar el resultado y corregir errores

`POST` devuelve `202` con `id` y links. Consultar el link de estado y luego el de resultado, con credenciales del mismo owner. El documento de resultado tiene `result`, `errors`, `errorSummary` y `roots`; los datos del árbol están dentro de `result`, bajo los nombres de sus raíces.

| Estado | Interpretación |
|---|---|
| `completed` | Terminó sin fallos observables de nodos. |
| `completed_with_errors` | Publica los datos disponibles y un informe de fallos. |
| `failed` | No tiene resultado publicable; revisar el error terminal. |

`pending` y `running` aún no son terminales. El endpoint de resultado responde `409` si todavía no está listo o la ejecución falló; `410` si expiró.

Una colección vacía válida es éxito. Una colección fallida se representa como `[]` y un `single` fallido como `null`, acompañados de diagnóstico cuando hay resultado parcial. Los fallos de enriquecimiento pueden dejar elementos `null` sin cambiar sus índices. Siempre leer `errors`, `errorSummary` y `roots` para distinguir ausencia legítima de datos de un fallo. El informe puede estar truncado por límites operativos; el resumen conserva los conteos.

| Síntoma | Revisar |
|---|---|
| Rechazo estructural/semántico | Propiedades admitidas, método GET, versión 1 y límites; ejecutar `validate`. |
| `SELECT_PATH_NOT_FOUND` | Forma real del body antes de seleccionar. |
| `BINDING_PATH_NOT_FOUND` | Campo del ancestro seleccionado/normalizado, no del body descartado por `select`. |
| `TYPE_MISMATCH` | Array vs. no-array, o parámetro URL no escalar. |
| `OUTPUT_FIELD_COLLISION` | Nombre de hijo que ya existe en el objeto original. |
| `PARENT_NOT_ENRICHABLE` | `null` o array anidado con hijos. |
| `QUOTA_POLICY_CONFLICT` | Declaraciones distintas para el mismo proveedor/endpoint compartido. |
| `EGRESS_DENIED` | Origin y destino autorizados por el operador. |
| `PAGINATION_SIZE_INCONSISTENT` | Tamaño pedido y cantidad realmente devuelta por el proveedor. |

Para contratos adicionales, consultar el [schema de resultados](../contracts/execution_result_v1.schema.json), la [matriz de pruebas](testing.md) y el [README operativo](../README.md).
