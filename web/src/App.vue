<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import MarkdownIt from 'markdown-it';
import serviceGuide from '../../docs/service-configuration.md?raw';
import requestGuide from '../../docs/api-request-guide.md?raw';
import serviceGuideEnglish from '../../docs/service-configuration.en.md?raw';
import requestGuideEnglish from '../../docs/api-request-guide.en.md?raw';
import ResultsGuide from './ResultsGuide.vue';
import pokemonExample from '../../docs/request_examples/poke_api.json?raw';
import { useI18n } from 'vue-i18n';
import { messagesFor } from './i18n';

type Config = Record<string, unknown>;
type Row = { path: string; description: string; defaultValue: unknown; currentValue: unknown };
const markdown = new MarkdownIt({ html: false, linkify: true, typographer: true });
const { locale, t } = useI18n();
const ui = messagesFor();
const uiLanguage = locale.value as 'es' | 'en';
const path = ref(location.pathname);
const username = ref(''), password = ref(''), error = ref(''), loading = ref(false);
const docsTabs = [
  { id: 'service', label: ui.service },
  { id: 'consumption', label: ui.consumption },
  { id: 'results', label: ui.results },
] as const;
const docsTab = ref<(typeof docsTabs)[number]['id']>('service');
function navigateDocsTabs(event: KeyboardEvent) {
  const current = docsTabs.findIndex((tab) => tab.id === docsTab.value);
  let next: number;
  switch (event.key) {
    case 'ArrowRight': next = (current + 1) % docsTabs.length; break;
    case 'ArrowLeft': next = (current + docsTabs.length - 1) % docsTabs.length; break;
    case 'Home': next = 0; break;
    case 'End': next = docsTabs.length - 1; break;
    default: return;
  }
  event.preventDefault();
  docsTab.value = docsTabs[next]!.id;
  const button = event.currentTarget as HTMLButtonElement;
  button.parentElement?.querySelectorAll<HTMLButtonElement>('[role="tab"]')[next]?.focus();
}
const configQuery = ref(''), requestQuery = ref(''), configError = ref('');
const effectiveConfig = ref<{ current: Config; defaults: Config }>();
const pokemonRequest = JSON.parse(pokemonExample) as Record<string, unknown>;
const demoWebhook = () => ({ url: `${location.origin}/playground/api/webhook`, headers: { 'X-API-Engine-Playground': 'demo' } });
const pokemonV1WebhookExample = JSON.stringify({ ...pokemonRequest, webhook: { ...demoWebhook(), payload: 'status' } }, null, 2);
const pokemonV2WebhookExample = JSON.stringify({ ...pokemonRequest, webhook: { ...demoWebhook(), ordering: 'completion', maxInFlight: 0 } }, null, 2);

const sections = [
  ['root', 'Documento'], ['http', 'HTTP'], ['executions', 'Ejecuciones'], ['fetch', 'Fetch'],
  ['throttle', 'Throttle'], ['deduplication', 'Deduplicación'], ['storage', 'Almacenamiento'],
  ['cache', 'Caché'], ['webhooks', 'Webhooks'], ['security', 'Seguridad'], ['maintenance', 'Mantenimiento'],
  ['shutdown', 'Apagado'], ['observability', 'Observabilidad'],
] as const;
const fields: Array<[string, string]> = [
  ['version', 'Versión del formato de configuración que interpreta el servicio.'],
  ['mode', 'Modo operativo y conjunto de drivers admitido por esta versión.'],
  ['ui.enabled', 'Activa o desactiva la consola web interactiva.'],
  ['ui.language', 'Idioma de los textos de la consola web: es o en.'],
  ['http.address', 'Dirección y puerto donde el proceso HTTP acepta conexiones.'],
  ['http.publicBaseUrl', 'URL pública usada en enlaces y para decidir si la cookie requiere HTTPS.'],
  ['http.readHeaderTimeout', 'Tiempo máximo para recibir las cabeceras HTTP.'],
  ['http.readTimeout', 'Tiempo máximo para leer una petición HTTP completa.'],
  ['http.writeTimeout', 'Tiempo máximo para escribir una respuesta HTTP.'],
  ['http.idleTimeout', 'Tiempo máximo de inactividad para una conexión keep-alive.'],
  ['http.maxRequestBytes', 'Tamaño máximo en bytes de un request de ejecución.'],
  ['executions.maxConcurrent', 'Cantidad máxima de ejecuciones procesadas simultáneamente.'],
  ['executions.timeout', 'Límite total de tiempo para una ejecución.'],
  ['executions.maxDefinitionDepth', 'Profundidad máxima permitida para el árbol de definición.'],
  ['executions.maxDefinitionNodes', 'Número máximo de nodos declarados en una definición.'],
  ['executions.maxChildrenPerNode', 'Número máximo de hijos que puede declarar un nodo.'],
  ['executions.maxRootCalls', 'Número máximo de llamadas raíz dentro de un request.'],
  ['executions.maxBranchGoroutines', 'Límite de ramas del árbol que pueden ejecutarse en paralelo.'],
  ['executions.maxPagesPerCall', 'Máximo de páginas que una llamada paginada puede seguir.'],
  ['executions.maxLogicalRequests', 'Máximo de solicitudes lógicas producidas por una ejecución.'],
  ['executions.maxHTTPAttempts', 'Máximo acumulado de intentos HTTP, incluidos reintentos.'],
  ['executions.maxResultNodes', 'Máximo de nodos serializables en el resultado v1.'],
  ['executions.maxResultBytes', 'Presupuesto máximo en bytes para el resultado completo v1.'],
  ['executions.detectRepeatedPages', 'Detiene una paginación cuando detecta una página repetida.'],
  ['executions.progressFlushInterval', 'Frecuencia con que se persiste y publica el avance.'],
  ['executions.errors.maxDetails', 'Número máximo de detalles incluidos en un reporte de errores.'],
  ['executions.errors.maxReportBytes', 'Tamaño máximo reservado para el reporte de errores.'],
  ['executions.errors.maxMessageBytes', 'Tamaño máximo de cada mensaje de error.'],
  ['executions.v2.maxPublishedBytes', 'Máximo de bytes que v2 puede publicar como eventos.'],
  ['executions.v2.maxStoredBytes', 'Máximo de bytes durables que v2 conserva por ejecución.'],
  ['executions.v2.maxChunks', 'Máximo de chunks que v2 puede publicar por ejecución.'],
  ['executions.v2.maxNodes', 'Máximo de nodos que v2 puede materializar.'],
  ['executions.v2.maxPendingTasks', 'Máximo de tareas pendientes aceptadas por v2.'],
  ['fetch.maxConcurrentHTTPRequests', 'Techo global de requests upstream simultáneos entre todas las ejecuciones; también es la concurrencia por defecto de api.limits cuando el request no la declara.'],
  ['fetch.connectTimeout', 'Tiempo máximo para establecer una conexión TCP saliente.'],
  ['fetch.tlsHandshakeTimeout', 'Tiempo máximo para completar el handshake TLS.'],
  ['fetch.responseHeaderTimeout', 'Tiempo máximo para recibir cabeceras de una respuesta remota.'],
  ['fetch.attemptTimeout', 'Tiempo máximo de un intento HTTP individual.'],
  ['fetch.maxResponseBytes', 'Tamaño máximo en bytes leído de una respuesta remota.'],
  ['fetch.maxJSONDepth', 'Profundidad máxima de JSON aceptada desde un proveedor remoto.'],
  ['fetch.maxJSONValuesPerResponse', 'Máximo de valores JSON procesados de una respuesta remota.'],
  ['fetch.maxIdleConnections', 'Número máximo de conexiones HTTP salientes inactivas.'],
  ['fetch.maxIdleConnectionsPerHost', 'Máximo de conexiones inactivas por host remoto.'],
  ['fetch.idleConnectionTimeout', 'Tiempo que una conexión saliente inactiva permanece reutilizable.'],
  ['fetch.retry.maxAttempts', 'Número máximo de intentos por solicitud HTTP saliente.'],
  ['fetch.retry.baseDelay', 'Demora inicial antes de reintentar una solicitud HTTP.'],
  ['fetch.retry.maxBackoff', 'Demora máxima entre reintentos HTTP.'],
  ['throttle.driver', 'Implementación del control de cuota usada por el motor.'],
  ['throttle.utilization', 'Proporción de la cuota declarada que el pacing puede consumir. Mantenerla debajo de 1 deja margen ante relojes y ventanas del proveedor.'],
  ['throttle.maxQuotaScopes', 'Máximo de ámbitos de cuota que el motor mantiene en memoria.'],
  ['throttle.coldStart', 'Política aplicada cuando aún no hay una ventana de cuota conocida.'],
  ['throttle.safetyMargin', 'Margen temporal que protege contra exceder una ventana de cuota.'],
  ['throttle.quotaGroups', 'Reglas del operador que definen la cuota efectiva compartida si el proveedor limita por cuenta, IP, varios tokens o varios orígenes. Prevalecen sobre api.limits.'],
  ['throttle.quotaGroups[].name', 'Nombre único que identifica el grupo de cuota.'],
  ['throttle.quotaGroups[].origins', 'Orígenes HTTPS que comparten la misma cuota.'],
  ['throttle.quotaGroups[].credentialMode', 'separate crea una cuota por credencial upstream; shared combina todas las credenciales en una sola cuota del grupo.'],
  ['throttle.quotaGroups[].limits.requests', 'Requests disponibles por ventana del grupo.'],
  ['throttle.quotaGroups[].limits.per', 'Duración de la ventana de cuota del grupo.'],
  ['throttle.quotaGroups[].limits.concurrency', 'Solicitudes del grupo permitidas simultáneamente.'],
  ['deduplication.driver', 'Implementación que une solicitudes idénticas en vuelo.'],
  ['deduplication.maxActiveFlights', 'Máximo de operaciones compartidas activas.'],
  ['deduplication.maxOperationTime', 'Tiempo máximo que una operación compartida permanece activa.'],
  ['storage.driver', 'Driver para los metadatos y resultados durables.'],
  ['storage.path', 'Ruta del archivo de base de datos de ejecuciones.'],
  ['storage.busyTimeout', 'Tiempo que SQLite espera antes de devolver una base ocupada.'],
  ['storage.maxOpenConnections', 'Máximo de conexiones abiertas a la base de ejecuciones.'],
  ['storage.synchronous', 'Nivel de sincronización de SQLite para la base de ejecuciones.'],
  ['storage.metadataRetention', 'Tiempo de retención de metadatos de ejecución.'],
  ['storage.resultRetention', 'Tiempo de retención de resultados de ejecución.'],
  ['storage.minimumFreeDiskBytes', 'Espacio libre mínimo requerido para aceptar trabajo nuevo.'],
  ['cache.driver', 'Implementación de la caché persistente de respuestas HTTP upstream; no almacena el grafo enriquecido final.'],
  ['cache.path', 'Ruta del archivo de base de datos de caché.'],
  ['cache.busyTimeout', 'Tiempo que SQLite espera antes de devolver una caché ocupada.'],
  ['cache.maxOpenConnections', 'Máximo de conexiones abiertas a la base de caché.'],
  ['cache.synchronous', 'Nivel de sincronización de SQLite para la caché.'],
  ['cache.maxEntryBytes', 'Tamaño máximo de una respuesta plaintext que se puede guardar en caché. Una respuesta mayor se usa pero no se conserva.'],
  ['cache.maxEntries', 'Máximo de entradas que puede contener la caché.'],
  ['cache.maxLogicalBytes', 'Presupuesto de bytes plaintext entre todas las entradas. Si se agota, se expulsan respuestas antiguas para conservar el límite.'],
  ['cache.maxTTL', 'TTL máximo que un request puede solicitar mediante api.cache o calls[].cache; no habilita caché por sí solo.'],
  ['webhooks.maxConcurrent', 'Entregas webhook que pueden ejecutarse al mismo tiempo.'],
  ['webhooks.maxPendingDeliveries', 'Máximo de entregas webhook pendientes que se conservan.'],
  ['webhooks.pollInterval', 'Frecuencia con que el worker busca entregas pendientes.'],
  ['webhooks.claimLease', 'Tiempo durante el cual un worker posee una entrega reclamada.'],
  ['webhooks.attemptTimeout', 'Tiempo máximo de un intento de entrega webhook.'],
  ['webhooks.maxResponseBytes', 'Máximo de bytes leídos de la respuesta de un webhook.'],
  ['webhooks.maxPayloadBytes', 'Tamaño máximo del payload enviado a un webhook.'],
  ['webhooks.maxAttempts', 'Número máximo de intentos de entrega webhook.'],
  ['webhooks.baseDelay', 'Demora inicial antes de reintentar una entrega webhook.'],
  ['webhooks.maxBackoff', 'Demora máxima entre reintentos webhook.'],
  ['webhooks.deliveryMaxAge', 'Edad máxima de una entrega antes de descartarla.'],
  ['security.authenticationEnabled', 'Activa la autenticación Bearer para la API pública.'],
  ['security.egress.upstreamAllowedOrigins', 'Orígenes autorizados para llamadas del grafo; admite *, exactos y *.dominio.'],
  ['security.egress.webhookAllowedOrigins', 'Orígenes autorizados para la entrega de webhooks.'],
  ['security.egress.allowedPrivateCIDRs', 'Redes privadas a las que el motor puede conectarse explícitamente.'],
  ['security.egress.allowHTTPForDevelopment', 'Permite destinos HTTP sin TLS para desarrollo local.'],
  ['maintenance.interval', 'Frecuencia con que se ejecutan tareas de mantenimiento.'],
  ['maintenance.batchSize', 'Número máximo de registros que procesa cada ciclo de mantenimiento.'],
  ['shutdown.gracePeriod', 'Tiempo para terminar trabajo en curso durante el apagado.'],
  ['shutdown.persistenceTimeout', 'Tiempo máximo para persistir estado crítico durante el apagado.'],
  ['observability.logLevel', 'Nivel mínimo de detalle emitido por los logs.'],
  ['observability.metricsEnabled', 'Expone o desactiva el endpoint de métricas.'],
  ['observability.metricsPath', 'Ruta HTTP donde se sirven las métricas.'],
];
type RequestField = { path: string; required: string; description: string; allowed: string };
const requestSections: Array<{ title: string; fields: RequestField[] }> = [
  { title: 'Envoltura', fields: [
    { path: 'definition', required: 'Sí', description: 'Definición del grafo, proveedor común y llamadas raíz.', allowed: 'Objeto' },
    { path: 'webhook', required: 'No', description: 'Notificaciones opcionales: cierre en V1; cadenas completas y cierre en V2. Puede omitirse o ser null.', allowed: 'Objeto o null' },
  ] },
  { title: 'definition', fields: [
    { path: 'definition.version', required: 'Sí', description: 'Versión del lenguaje de grafos.', allowed: '1' },
    { path: 'definition.api', required: 'Sí', description: 'Proveedor, autenticación, cuota y caché comunes.', allowed: 'Objeto' },
    { path: 'definition.calls', required: 'Sí', description: 'Raíces independientes del grafo; incluso una sola debe ir en un arreglo.', allowed: 'Array no vacío de nodos' },
  ] },
  { title: 'definition.api', fields: [
    { path: 'api.baseUrl', required: 'Sí', description: 'URL base HTTPS del proveedor, con prefijo de path opcional.', allowed: 'URI absoluta' },
    { path: 'api.limits.requests', required: 'Sí', description: 'Requests declarados por ventana para el proveedor. El engine espacía intentos físicos; retries y paginación también consumen esta cuota.', allowed: 'Entero ≥ 1' },
    { path: 'api.limits.per', required: 'Sí', description: 'Duración de la ventana de cuota. Junto con requests define el pacing del proveedor.', allowed: 'Duración, ej. 1m' },
    { path: 'api.limits.concurrency', required: 'No', description: 'Máximo de solicitudes del proveedor en vuelo. Si se omite, se usa fetch.maxConcurrentHTTPRequests del operador.', allowed: 'Entero ≥ 1' },
    { path: 'api.auth', required: 'No', description: 'Credenciales que el engine usa con el proveedor.', allowed: 'none, bearer, basic o header' },
    { path: 'api.headers', required: 'No', description: 'Headers estáticos aplicados a cada llamada del proveedor.', allowed: 'Mapa string → string' },
    { path: 'api.cache', required: 'No', description: 'Política de caché base para las raíces. Omitirla o usar false evita guardar respuestas; los nodos heredan la política hasta que la reemplacen.', allowed: 'false o { ttl, ignoreServerCachePolicy? }' },
  ] },
  { title: 'Nodo calls[]', fields: [
    { path: 'calls[].name', required: 'Sí', description: 'Nombre de la propiedad de salida; es único entre hermanos.', allowed: 'Identificador de 1–64 caracteres' },
    { path: 'calls[].type', required: 'Sí', description: 'Forma esperada del valor seleccionado.', allowed: 'single o collection' },
    { path: 'calls[].select', required: 'No', description: 'Ruta por puntos sobre el JSON remoto; omitido usa el body completo.', allowed: 'String, null u omitido' },
    { path: 'calls[].request', required: 'Sí', description: 'Solicitud GET que resuelve el nodo.', allowed: 'Objeto request' },
    { path: 'calls[].children', required: 'No', description: 'Nodos que enriquecen cada valor del nodo actual.', allowed: 'Array de nodos' },
    { path: 'calls[].pagination', required: 'No', description: 'Paginación por número de página; solo collection.', allowed: 'Objeto pagination' },
    { path: 'calls[].limits', required: 'No', description: 'Límite adicional del endpoint. Se combina con la cuota efectiva del proveedor y no limita automáticamente a hijos en otros endpoints.', allowed: 'Objeto limits' },
    { path: 'calls[].cache', required: 'No', description: 'Reemplaza la política heredada para este nodo y sus hijos. false no lee ni escribe caché; un hijo puede volver a habilitar su propio ttl.', allowed: 'false o { ttl, ignoreServerCachePolicy? }' },
  ] },
  { title: 'request', fields: [
    { path: 'request.method', required: 'Sí', description: 'Método HTTP de una llamada de grafo.', allowed: 'GET' },
    { path: 'request.path', required: 'Sí', description: 'Ruta relativa al baseUrl; admite bindings de parent y ancestros.', allowed: 'Empieza con /; sin query ni fragmento' },
    { path: 'request.query', required: 'No', description: 'Parámetros query estáticos o con bindings en valores string.', allowed: 'Mapa string → string, número o boolean' },
    { path: 'request.headers', required: 'No', description: 'Headers que reemplazan los headers de api por nombre.', allowed: 'Mapa string → string' },
  ] },
  { title: 'pagination', fields: [
    { path: 'pagination.page', required: 'Sí', description: 'Nombre del parámetro query que contiene el número de página.', allowed: 'String' },
    { path: 'pagination.pageSize', required: 'No', description: 'Nombre del parámetro query que contiene el tamaño de página.', allowed: 'String' },
    { path: 'pagination.start', required: 'No', description: 'Número de la primera página.', allowed: 'Entero ≥ 0; default 1' },
    { path: 'pagination.pages', required: 'No', description: 'Máximo de páginas que puede seguir el nodo.', allowed: 'all o entero ≥ 1; default all' },
  ] },
  { title: 'Autenticación, caché y webhook', fields: [
    { path: 'api.auth.type', required: 'No', description: 'Tipo de autenticación de upstream.', allowed: 'none, bearer, basic o header' },
    { path: 'api.auth.token', required: 'Según tipo', description: 'Token para auth bearer.', allowed: 'String no vacío' },
    { path: 'api.auth.username / password', required: 'Según tipo', description: 'Credenciales para auth basic.', allowed: 'Strings no vacíos' },
    { path: 'api.auth.name / value', required: 'Según tipo', description: 'Nombre y valor para auth header.', allowed: 'Strings no vacíos' },
    { path: 'cache.ttl', required: 'Sí si cache es objeto', description: 'Tiempo durante el que la respuesta HTTP base se considera fresca. No cachea el resultado enriquecido completo y no puede superar cache.maxTTL.', allowed: 'Duración, ej. 5m' },
    { path: 'cache.ignoreServerCachePolicy', required: 'No', description: 'Si es true, usa cache.ttl aunque Cache-Control, Expires, Age o Date del proveedor indiquen otra política. Por defecto es false; Set-Cookie y Vary: * siguen bloqueando la caché.', allowed: 'Booleano; default false' },
    { path: 'webhook.url', required: 'Sí si webhook existe', description: 'Destino de las notificaciones: cierre en V1; nodos terminados y cierre en V2.', allowed: 'URI absoluta' },
    { path: 'webhook.method', required: 'No', description: 'Método de entrega del webhook.', allowed: 'POST, PUT o PATCH; default POST' },
    { path: 'webhook.headers', required: 'No', description: 'Headers adicionales del webhook.', allowed: 'Mapa string → string' },
    { path: 'webhook.auth', required: 'No', description: 'Autenticación para el destino webhook.', allowed: 'none, bearer, basic o header' },
    { path: 'webhook.payload', required: 'No', description: 'En V1 elige el contenido terminal. En V2 los eventos de nodo incluyen su resultado automáticamente.', allowed: 'V1: status o result. V2: sólo status. Default: status' },
    { path: 'webhook.ordering', required: 'No; sólo V2', description: 'Orden de admisión de webhooks: por finalización o posición original. No cambia la disponibilidad por API; con varias entregas no garantiza orden de recepción.', allowed: 'completion o source; default completion' },
    { path: 'webhook.maxInFlight', required: 'No; sólo V2', description: 'Entregas pendientes de resolución simultáneamente. Con 1 espera confirmación 2xx o fallo definitivo; con 0 no hay límite por ejecución. Los reintentos conservan su lugar. Siempre aplica webhooks.maxConcurrent global.', allowed: 'Entero ≥ 0; 0 ilimitado; default 1' },
  ] },
];
const requestSchemaExample = JSON.stringify({
  definition: {
    version: 1,
    api: { baseUrl: 'https://example.com/api/v1', limits: { requests: 100, per: '1m' } },
    calls: [{ name: 'products', type: 'collection', select: 'data.items', request: { method: 'GET', path: '/products' } }],
  },
}, null, 2);
function valueAt(source: unknown, field: string): unknown {
  if (field.includes('[]')) return undefined;
  return field.split('.').reduce<unknown>((value, key) => value && typeof value === 'object' && !Array.isArray(value) ? (value as Config)[key] : undefined, source);
}
function display(value: unknown, perItem = false): string {
  if (perItem) return t('common.definedPerItem');
  if (value === undefined) return '—';
  return typeof value === 'string' ? (value || t('common.empty')) : JSON.stringify(value);
}
function fieldSection(field: string) { return field.includes('.') ? field.split('.')[0] : 'root'; }
const sectionTitlesEnglish: Record<string, string> = { root: 'Document', http: 'HTTP', executions: 'Executions', fetch: 'Fetch', throttle: 'Throttling', deduplication: 'Deduplication', storage: 'Storage', cache: 'Cache', webhooks: 'Webhooks', security: 'Security', maintenance: 'Maintenance', shutdown: 'Shutdown', observability: 'Observability' };
const englishFieldDescriptions: Record<string, string> = {
  version: 'Configuration document format understood by this engine version.',
  mode: 'Operating mode and the supported driver set. This release runs locally with SQLite-backed state.',
  'ui.enabled': 'Enables the authenticated web console. Disabling it does not disable the API, workers, health checks, or metrics.',
  'ui.language': 'Language selected at startup for the console, including this documentation: es or en.',
  'http.address': 'Interface and port on which the process accepts HTTP connections.',
  'http.publicBaseUrl': 'Public URL used in generated links and to decide whether session cookies require HTTPS.',
  'http.readHeaderTimeout': 'Maximum time allowed to receive request headers.',
  'http.readTimeout': 'Maximum time allowed to read one complete inbound request.',
  'http.writeTimeout': 'Maximum time allowed to send an HTTP response.',
  'http.idleTimeout': 'How long an idle keep-alive connection remains open.',
  'http.maxRequestBytes': 'Largest execution-definition body the API accepts before validation.',
  'executions.maxConcurrent': 'Maximum number of executions the worker processes at the same time.',
  'executions.timeout': 'Absolute deadline for one execution, including quota waits and retries.',
  'executions.maxDefinitionDepth': 'Maximum nesting depth of the submitted call graph.',
  'executions.maxDefinitionNodes': 'Maximum number of call nodes declared in one graph.',
  'executions.maxChildrenPerNode': 'Maximum child calls declared below a single node.',
  'executions.maxRootCalls': 'Maximum independent root calls in one execution.',
  'executions.maxBranchGoroutines': 'Maximum graph branches that may run in parallel within one execution.',
  'executions.maxPagesPerCall': 'Hard stop for pages followed by one paginated call.',
  'executions.maxLogicalRequests': 'Maximum logical node and page requests produced by one execution, including cache hits.',
  'executions.maxHTTPAttempts': 'Maximum physical upstream attempts for one execution, including retries.',
  'executions.maxResultNodes': 'Maximum result values and objects materialized by a V1 execution.',
  'executions.maxResultBytes': 'Byte budget for the assembled V1 result document.',
  'executions.detectRepeatedPages': 'Stops a paginator when it observes a repeated page instead of looping forever.',
  'executions.progressFlushInterval': 'How often running progress is persisted and made visible to readers.',
  'executions.errors.maxDetails': 'Maximum diagnostic entries retained in one execution report.',
  'executions.errors.maxReportBytes': 'Byte budget reserved for the public error report.',
  'executions.errors.maxMessageBytes': 'Maximum size of a single diagnostic message.',
  'executions.v2.maxPublishedBytes': 'Maximum bytes V2 may publish as result events for one execution.',
  'executions.v2.maxStoredBytes': 'Maximum durable V2 result bytes retained for one execution.',
  'executions.v2.maxChunks': 'Maximum low-level chunks V2 may persist and publish.',
  'executions.v2.maxNodes': 'Maximum V2 result nodes that may be materialized.',
  'executions.v2.maxPendingTasks': 'Maximum unfinished V2 tasks accepted for one execution.',
  'fetch.maxConcurrentHTTPRequests': 'Global ceiling for concurrent upstream requests across every execution. It is also the default provider concurrency when a request omits api.limits.concurrency.',
  'fetch.connectTimeout': 'Maximum time to establish an upstream TCP connection.',
  'fetch.tlsHandshakeTimeout': 'Maximum time to complete the upstream TLS handshake.',
  'fetch.responseHeaderTimeout': 'Maximum time to receive upstream response headers after writing a request.',
  'fetch.attemptTimeout': 'Maximum duration of one physical upstream attempt.',
  'fetch.maxResponseBytes': 'Largest upstream response body read before the attempt fails.',
  'fetch.maxJSONDepth': 'Maximum JSON nesting accepted from an upstream response.',
  'fetch.maxJSONValuesPerResponse': 'Maximum JSON values processed from one upstream response.',
  'fetch.maxIdleConnections': 'Pool-wide maximum number of idle outbound HTTP connections.',
  'fetch.maxIdleConnectionsPerHost': 'Maximum reusable idle outbound connections kept for one host.',
  'fetch.idleConnectionTimeout': 'How long an unused outbound connection remains reusable.',
  'fetch.retry.maxAttempts': 'Maximum physical attempts for one uncached upstream request.',
  'fetch.retry.baseDelay': 'Initial randomized delay used before retrying a retryable upstream failure.',
  'fetch.retry.maxBackoff': 'Upper bound for the delay between upstream retries.',
  'throttle.driver': 'Quota-admission implementation. local_pacing coordinates requests in this engine process.',
  'throttle.utilization': 'Fraction of a declared rate that pacing may use. Keeping this below 1 leaves room for clock and provider-window differences.',
  'throttle.maxQuotaScopes': 'Maximum provider, endpoint, and managed quota scopes kept in the local gate.',
  'throttle.coldStart': 'Conservative policy used before this process has observed a quota window.',
  'throttle.safetyMargin': 'Extra wait added to rate windows and cold starts to reduce accidental limit overruns.',
  'throttle.quotaGroups': 'Operator-owned rules that set the effective shared limit when a provider quota spans tokens, consumers, or origins. They take precedence over api.limits.',
  'throttle.quotaGroups[].name': 'Stable, unique name for one managed provider quota.',
  'throttle.quotaGroups[].origins': 'Exact upstream origins governed by this shared quota.',
  'throttle.quotaGroups[].credentialMode': 'separate creates one group scope per upstream credential; shared combines every credential into one scope.',
  'throttle.quotaGroups[].limits.requests': 'Maximum physical requests admitted by this managed group per period.',
  'throttle.quotaGroups[].limits.per': 'Period used by the managed group rate limit.',
  'throttle.quotaGroups[].limits.concurrency': 'Maximum simultaneous requests admitted by this managed group.',
  'deduplication.driver': 'Implementation that lets identical requests already in flight share one physical upstream call.',
  'deduplication.maxActiveFlights': 'Maximum number of distinct shared operations active at once.',
  'deduplication.maxOperationTime': 'Longest time a shared operation may remain active before it is stopped.',
  'storage.driver': 'Durable execution metadata and result storage driver.',
  'storage.path': 'Path to the SQLite database containing execution state, results, webhooks, and cooldowns.',
  'storage.busyTimeout': 'How long SQLite waits for the execution database to become writable.',
  'storage.maxOpenConnections': 'Maximum open connections to the execution database.',
  'storage.synchronous': 'SQLite durability mode for execution state. FULL protects durable state before acknowledgement.',
  'storage.metadataRetention': 'How long execution metadata remains available after completion.',
  'storage.resultRetention': 'How long execution results remain available before cleanup.',
  'storage.minimumFreeDiskBytes': 'Free-disk reserve required before accepting new work; protects persistence of active work.',
  'cache.driver': 'Persistent response-cache implementation.',
  'cache.path': 'Path to the SQLite database that stores reusable upstream HTTP responses.',
  'cache.busyTimeout': 'How long SQLite waits for the cache database to become writable.',
  'cache.maxOpenConnections': 'Maximum open connections to the cache database.',
  'cache.synchronous': 'SQLite durability mode used for cache entries. Cache can use a lighter mode than execution state.',
  'cache.maxEntryBytes': 'Largest plaintext response eligible for caching.',
  'cache.maxEntries': 'Maximum number of reusable HTTP responses retained.',
  'cache.maxLogicalBytes': 'Logical plaintext byte budget across all cached responses; older entries are evicted when needed.',
  'cache.maxTTL': 'Longest freshness duration a request may ask for with api.cache or calls[].cache.',
  'webhooks.maxConcurrent': 'Global ceiling for webhook deliveries in flight; separate from upstream fetch concurrency.',
  'webhooks.maxPendingDeliveries': 'Maximum durable webhook deliveries waiting to be sent or retried.',
  'webhooks.pollInterval': 'How often the dispatcher looks for deliveries that are ready to send.',
  'webhooks.claimLease': 'Time a worker owns a claimed delivery before it may be recovered after interruption.',
  'webhooks.attemptTimeout': 'Maximum duration of one webhook delivery attempt.',
  'webhooks.maxResponseBytes': 'Largest response body read from a webhook receiver.',
  'webhooks.maxPayloadBytes': 'Largest JSON payload the engine will send to one webhook receiver.',
  'webhooks.maxAttempts': 'Maximum delivery attempts before a webhook delivery becomes terminally failed.',
  'webhooks.baseDelay': 'Initial randomized delay before retrying a webhook delivery.',
  'webhooks.maxBackoff': 'Upper bound for webhook retry delay.',
  'webhooks.deliveryMaxAge': 'Oldest a pending webhook delivery may become before it expires.',
  'security.authenticationEnabled': 'Requires bearer authentication for the public execution API. It may only be disabled for loopback development.',
  'security.egress.upstreamAllowedOrigins': 'Only provider origins on this allowlist may be called by execution graphs.',
  'security.egress.webhookAllowedOrigins': 'Only webhook receiver origins on this allowlist may receive execution notifications.',
  'security.egress.allowedPrivateCIDRs': 'Private networks explicitly allowed as upstream or webhook destinations.',
  'security.egress.allowHTTPForDevelopment': 'Permits non-TLS HTTP destinations for local development; it should remain false in production.',
  'maintenance.interval': 'Frequency of cleanup and maintenance work, including expired cache entries and cooldown records.',
  'maintenance.batchSize': 'Maximum records processed by each maintenance pass, keeping cleanup work bounded.',
  'shutdown.gracePeriod': 'Time allowed for in-flight work to finish after shutdown begins.',
  'shutdown.persistenceTimeout': 'Separate time budget to persist critical state while shutting down.',
  'observability.logLevel': 'Minimum log severity emitted by the service.',
  'observability.metricsEnabled': 'Enables the protected metrics endpoint.',
  'observability.metricsPath': 'HTTP path where the service exposes metrics.',
};
const englishRequestSectionTitles: Record<string, string> = { Envoltura: 'Envelope', definition: 'definition', 'definition.api': 'definition.api', 'Nodo calls[]': 'calls[] node', request: 'request', pagination: 'pagination', 'Autenticación, caché y webhook': 'Authentication, cache, and webhook' };
const englishRequestFields: Record<string, Pick<RequestField, 'required' | 'description' | 'allowed'>> = {
  definition: { required: 'Yes', description: 'The complete declarative graph: one provider, its shared policy, and the root calls to execute.', allowed: 'Object' },
  webhook: { required: 'No', description: 'Optional execution notifications. V1 sends a terminal event; V2 sends completed chains and a terminal event.', allowed: 'Object or null' },
  'definition.version': { required: 'Yes', description: 'Version of the graph language. The execution API version is selected by the URL, not this field.', allowed: '1' },
  'definition.api': { required: 'Yes', description: 'Shared provider connection, credentials, rate policy, headers, and cache policy for every graph node.', allowed: 'Object' },
  'definition.calls': { required: 'Yes', description: 'Independent graph roots. A single root must still be wrapped in an array.', allowed: 'Non-empty node array' },
  'api.baseUrl': { required: 'Yes', description: 'Absolute provider base URL. A path prefix is allowed; query strings, fragments, and URL credentials are not.', allowed: 'Absolute URI' },
  'api.limits.requests': { required: 'Yes', description: 'Declared provider requests per period. The engine paces physical attempts rather than bursting this amount.', allowed: 'Integer ≥ 1' },
  'api.limits.per': { required: 'Yes', description: 'Period for the provider rate limit, for example 1m. Together with requests, this applies to retries and pagination.', allowed: 'Duration, e.g. 1m' },
  'api.limits.concurrency': { required: 'No', description: 'Maximum provider requests in flight. If omitted, the operator fetch concurrency default applies.', allowed: 'Integer ≥ 1' },
  'api.auth': { required: 'No', description: 'Credentials used by the engine when it calls the provider. They also identify the default shared provider quota.', allowed: 'none, bearer, basic, or header' },
  'api.headers': { required: 'No', description: 'Static headers attached to every provider request. Node headers replace them by header name.', allowed: 'string → string map' },
  'api.cache': { required: 'No', description: 'Default response-cache policy for root nodes. Omit or use false to avoid storing responses; child nodes inherit it unless they override it.', allowed: 'false or { ttl, ignoreServerCachePolicy? }' },
  'calls[].name': { required: 'Yes', description: 'Output property name. It must be unique among siblings and must not overwrite a returned object property.', allowed: '1–64 character identifier' },
  'calls[].type': { required: 'Yes', description: 'Expected shape after select: single accepts one value; collection accepts an array and runs its children for each item.', allowed: 'single or collection' },
  'calls[].select': { required: 'No', description: 'Dotted path evaluated against the provider JSON. Omit it or use null to keep the whole response body.', allowed: 'String, null, or omitted' },
  'calls[].request': { required: 'Yes', description: 'GET request that resolves this node.', allowed: 'request object' },
  'calls[].children': { required: 'No', description: 'Calls that enrich every selected parent value. They can bind values from the parent and named ancestors.', allowed: 'Node array' },
  'calls[].pagination': { required: 'No', description: 'Page-number pagination for a collection node. It is bounded by both pages and service limits.', allowed: 'pagination object' },
  'calls[].limits': { required: 'No', description: 'Additional endpoint-specific rate or concurrency restriction combined with the effective provider quota.', allowed: 'limits object' },
  'calls[].cache': { required: 'No', description: 'Overrides the inherited cache policy for this node and its descendants. false prevents cache reads and writes here; a child may enable its own TTL again.', allowed: 'false or { ttl, ignoreServerCachePolicy? }' },
  'request.method': { required: 'Yes', description: 'HTTP method used by a graph call. This graph language deliberately permits only safe GET requests.', allowed: 'GET' },
  'request.path': { required: 'Yes', description: 'Path relative to api.baseUrl. It supports parent and ancestor bindings, but never a complete URL, query string, or fragment.', allowed: 'Starts with /; no query or fragment' },
  'request.query': { required: 'No', description: 'Static query values or string bindings. Repeated keys, arrays, objects, and null are not supported.', allowed: 'string, number, or boolean map' },
  'request.headers': { required: 'No', description: 'Static headers for this node. Same-name headers replace api.headers without case sensitivity.', allowed: 'string → string map' },
  'pagination.page': { required: 'Yes', description: 'Name of the query parameter the engine increments for each page.', allowed: 'String' },
  'pagination.pageSize': { required: 'No', description: 'Name of a static integer query parameter that declares the requested page size.', allowed: 'String' },
  'pagination.start': { required: 'No', description: 'First page number requested by the engine.', allowed: 'Integer ≥ 0; default 1' },
  'pagination.pages': { required: 'No', description: 'Maximum page count this node may follow, not the final page number. all is still constrained by service limits.', allowed: 'all or integer ≥ 1; default all' },
  'api.auth.type': { required: 'No', description: 'Provider authentication scheme. Omission is equivalent to none.', allowed: 'none, bearer, basic, or header' },
  'api.auth.token': { required: 'Depending on type', description: 'Bearer token sent to the provider. It is not written into execution results.', allowed: 'Non-empty string' },
  'api.auth.username / password': { required: 'Depending on type', description: 'Basic-auth credentials sent to the provider.', allowed: 'Non-empty strings' },
  'api.auth.name / value': { required: 'Depending on type', description: 'Header name and value used by header authentication.', allowed: 'Non-empty strings' },
  'cache.ttl': { required: 'Yes when cache is an object', description: 'Freshness duration for the base HTTP response. It does not cache the completed enriched graph.', allowed: 'Duration, e.g. 5m' },
  'cache.ignoreServerCachePolicy': { required: 'No', description: 'When true, uses cache.ttl despite provider Cache-Control, Expires, Age, or Date. Defaults to false; Set-Cookie and Vary: * still prevent caching.', allowed: 'Boolean; default false' },
  'webhook.url': { required: 'Yes when webhook exists', description: 'Allowed receiver URL for execution notifications.', allowed: 'Absolute URI' },
  'webhook.method': { required: 'No', description: 'HTTP method used to deliver a webhook.', allowed: 'POST, PUT, or PATCH; default POST' },
  'webhook.headers': { required: 'No', description: 'Additional static receiver headers. Reserved delivery headers cannot be set.', allowed: 'string → string map' },
  'webhook.auth': { required: 'No', description: 'Authentication used when delivering to the webhook receiver.', allowed: 'none, bearer, basic, header, or hmac' },
  'webhook.payload': { required: 'No', description: 'V1 chooses the terminal payload. V2 node events always carry their completed value and accepts status only for the terminal payload.', allowed: 'V1: status or result. V2: status only. Default: status' },
  'webhook.ordering': { required: 'No; V2 only', description: 'Webhook admission order: completion or source position. It does not change API result availability or guarantee receiver arrival order.', allowed: 'completion or source; default completion' },
  'webhook.maxInFlight': { required: 'No; V2 only', description: 'Unresolved deliveries allowed concurrently for this execution. 1 waits for a 2xx or terminal failure; 0 removes only this per-execution limit.', allowed: 'Integer ≥ 0; default 1' },
};
const configRows = computed<Row[]>(() => fields.map(([field, description]) => ({ path: field, description: uiLanguage === 'en' ? englishFieldDescriptions[field] ?? description : description, defaultValue: valueAt(effectiveConfig.value?.defaults, field), currentValue: valueAt(effectiveConfig.value?.current, field) })));
const visibleSections = computed(() => {
  const query = configQuery.value.trim().toLowerCase();
  return sections.map(([id, title]) => ({ id, title: uiLanguage === 'en' ? sectionTitlesEnglish[id] ?? title : title, rows: configRows.value.filter((row) => fieldSection(row.path) === id && (!query || `${row.path} ${row.description} ${display(row.currentValue)}`.toLowerCase().includes(query))) })).filter((section) => section.rows.length);
});
const visibleRequestSections = computed(() => {
  const query = requestQuery.value.trim().toLowerCase();
  return requestSections.map((section) => ({ ...section, title: uiLanguage === 'en' ? englishRequestSectionTitles[section.title] ?? section.title : section.title, fields: section.fields.map((field) => uiLanguage === 'en' ? { ...field, ...(englishRequestFields[field.path] ?? {}) } : field).filter((field) => !query || `${field.path} ${field.description} ${field.allowed}`.toLowerCase().includes(query)) })).filter((section) => section.fields.length);
});
const renderedService = computed(() => markdown.render(uiLanguage === 'en' ? serviceGuideEnglish : serviceGuide));
const renderedConsumption = computed(() => markdown.render(uiLanguage === 'en' ? requestGuideEnglish : requestGuide));
async function loadConfiguration() {
  try { const response = await fetch('/playground/api/configuration'); if (!response.ok) throw new Error(); effectiveConfig.value = await response.json() as { current: Config; defaults: Config }; }
  catch { configError.value = 'No se pudo leer la configuración efectiva del servicio.'; }
}
async function login() {
  loading.value = true; error.value = '';
  try { const response = await fetch('/playground/api/session', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ username: username.value, password: password.value }) }); if (response.ok) { location.assign('/playground'); return; } error.value = response.status === 401 ? 'Credenciales incorrectas.' : 'No se pudo iniciar sesión. Intenta nuevamente.'; }
  catch { error.value = 'No se pudo conectar con el servicio.'; } finally { loading.value = false; }
}
async function logout() { await fetch('/playground/api/session', { method: 'DELETE' }); location.assign('/login'); }
function tryExample(example: string) { location.assign(`/playground?example=${encodeURIComponent(example)}`); }
onMounted(async () => { if (path.value !== '/login' && !(await fetch('/playground/api/session')).ok) { location.replace('/login'); return; } if (path.value === '/docs') await loadConfiguration(); });
</script>

<template>
  <main class="shell">
    <header><a href="/playground" class="brand">↳ API ENGINE <small>v2</small></a><nav v-if="path !== '/login'"><a href="/playground">{{ ui.playground }}</a><a href="/docs">{{ ui.docs }}</a><a href="/examples">{{ ui.examples }}</a><button @click="logout">{{ ui.logout }}</button></nav></header>
    <section v-if="path === '/login'" class="login"><p class="eyebrow">{{ ui.loginEyebrow }}</p><h1>{{ ui.loginTitle }}</h1><p>{{ ui.loginLead }}</p><form @submit.prevent="login"><label>{{ uiLanguage === 'en' ? 'Username' : 'Usuario' }}<input v-model="username" autocomplete="username" required></label><label>{{ uiLanguage === 'en' ? 'Password' : 'Contraseña' }}<input v-model="password" type="password" autocomplete="current-password" required></label><p v-if="error" class="error" role="alert">{{ error }}</p><button :disabled="loading">{{ loading ? (uiLanguage === 'en' ? 'Validating…' : 'Validando…') : ui.loginButton }}</button></form></section>
    <section v-else-if="path === '/docs'" class="documentation"><p class="eyebrow">{{ ui.docsEyebrow }}</p><h1>{{ ui.docsTitle }}</h1><p class="lead">{{ ui.docsLead }}</p><div class="tabs" role="tablist" :aria-label="ui.docs">
        <button v-for="tab in docsTabs" :id="'docs-tab-' + tab.id" :key="tab.id" role="tab" :aria-selected="docsTab === tab.id" :aria-controls="'docs-panel-' + tab.id" :tabindex="docsTab === tab.id ? 0 : -1" :class="{ active: docsTab === tab.id }" @click="docsTab = tab.id" @keydown="navigateDocsTabs">{{ tab.label }}</button>
      </div>
      <div :id="'docs-panel-' + docsTab" role="tabpanel" :aria-labelledby="'docs-tab-' + docsTab" tabindex="0">
      <template v-if="docsTab === 'service'"><section class="configuration"><div class="section-heading"><div><h2>{{ uiLanguage === 'en' ? 'Effective configuration' : 'Configuración efectiva' }}</h2><p>{{ uiLanguage === 'en' ? 'Values loaded at startup. Credentials, keys, and secret paths are not exposed.' : 'Valores cargados al arrancar. Las credenciales, claves y rutas de secretos no se exponen.' }}</p></div><input v-model="configQuery" :placeholder="uiLanguage === 'en' ? 'Search property or value…' : 'Buscar propiedad o valor…'" :aria-label="uiLanguage === 'en' ? 'Search configuration property' : 'Buscar propiedad de configuración'"></div><p v-if="configError" class="error">{{ configError }}</p><p v-else-if="!effectiveConfig" class="muted">{{ uiLanguage === 'en' ? 'Loading configuration…' : 'Cargando configuración…' }}</p><div v-else class="configuration-sections"><section v-for="section in visibleSections" :key="section.id" class="configuration-section"><h3>{{ section.title }}</h3><div class="table-wrap"><table><thead><tr><th>{{ uiLanguage === 'en' ? 'Property' : 'Propiedad' }}</th><th>{{ uiLanguage === 'en' ? 'Purpose' : 'Para qué sirve' }}</th><th>Default</th><th>{{ uiLanguage === 'en' ? 'Current value' : 'Valor actual' }}</th></tr></thead><tbody><tr v-for="row in section.rows" :key="row.path"><td><code>{{ row.path }}</code></td><td>{{ row.description }}</td><td><code>{{ display(row.defaultValue, row.path.includes('[]')) }}</code></td><td><code class="current">{{ display(row.currentValue, row.path.includes('[]')) }}</code></td></tr></tbody></table></div></section></div></section><article class="markdown service-guide" v-html="renderedService"></article></template>
      <template v-else-if="docsTab === 'consumption'"><section class="request-guide"><div class="request-intro"><div><h2>{{ uiLanguage === 'en' ? 'Build a declarative graph.' : 'Construye un grafo declarativo.' }}</h2><p>{{ uiLanguage === 'en' ? 'A request describes a provider and a tree or forest of GET calls. Each node selects a response and can enrich it with children that use parent or ancestor data.' : 'Un request describe un proveedor y un árbol o bosque de llamadas GET. Cada nodo selecciona una respuesta y puede enriquecerla con hijos que usan datos de su padre o de un ancestro.' }}</p><p>{{ uiLanguage === 'en' ? 'The structure is validated before execution; egress, quota, and service limits are validated when submitted.' : 'La estructura se valida antes de ejecutar; las políticas de egress, cuotas y límites del servicio se validan al enviarla.' }}</p></div><span class="schema-link">{{ uiLanguage === 'en' ? 'V1 contract' : 'Contrato v1' }}<br>JSON Schema Draft 2020-12</span></div><section class="schema-example"><div><h3>{{ uiLanguage === 'en' ? 'Minimal valid request' : 'Request mínimo válido' }}</h3><p>{{ uiLanguage === 'en' ? 'A collection root node against one provider. Copy this starting point and adapt the URL, quota, selector, and path.' : 'Un nodo raíz de colección contra un solo proveedor. Copia este punto de partida y adapta la URL, cuota, selector y ruta.' }}</p></div><pre :aria-label="uiLanguage === 'en' ? 'Valid request example' : 'Ejemplo de request válido'">{{ requestSchemaExample }}</pre></section><section class="request-fields"><div class="section-heading"><div><h2>{{ uiLanguage === 'en' ? 'Request fields' : 'Campos del request' }}</h2><p>{{ uiLanguage === 'en' ? 'Search a property, rule, or allowed value. Tables follow the order in which the JSON is composed.' : 'Busca una propiedad, regla o valor permitido. Las tablas siguen el orden en que se compone el JSON.' }}</p></div><input v-model="requestQuery" :placeholder="uiLanguage === 'en' ? 'Search request parameter…' : 'Buscar parámetro de request…'" :aria-label="uiLanguage === 'en' ? 'Search request parameter' : 'Buscar parámetro de request'"></div><div class="configuration-sections"><section v-for="section in visibleRequestSections" :key="section.title" class="configuration-section"><h3>{{ section.title }}</h3><div class="table-wrap"><table class="request-table"><thead><tr><th>{{ uiLanguage === 'en' ? 'Property' : 'Propiedad' }}</th><th>{{ uiLanguage === 'en' ? 'Required' : 'Obligatorio' }}</th><th>{{ uiLanguage === 'en' ? 'Purpose' : 'Para qué sirve' }}</th><th>{{ uiLanguage === 'en' ? 'Values and rule' : 'Valores y regla' }}</th></tr></thead><tbody><tr v-for="field in section.fields" :key="field.path"><td><code>{{ field.path }}</code></td><td><span class="requirement">{{ field.required }}</span></td><td>{{ field.description }}</td><td><code>{{ field.allowed }}</code></td></tr></tbody></table></div></section></div></section></section><article class="markdown service-guide" v-html="renderedConsumption"></article></template>
      <ResultsGuide v-else />
      </div>
    </section>
    <section v-else-if="path === '/examples'" class="documentation"><p class="eyebrow">{{ ui.examplesEyebrow }}</p><h1>{{ ui.examplesTitle }}</h1><p class="lead">{{ uiLanguage === 'en' ? 'All three examples query the same related graph. The buttons load the request into the playground; V1 and V2 webhooks target the current host.' : 'Los tres ejemplos consultan el mismo bosque con relaciones. Los botones cargan el request en el playground; el webhook de V1 y V2 apunta al receptor fijo del host actual.' }}</p><div class="example-grid examples-grid"><article><span class="example-version">BASE</span><h2>{{ uiLanguage === 'en' ? 'Basic request' : 'Request básico' }}</h2><p>{{ uiLanguage === 'en' ? 'The starting point without a webhook. Query the result from the viewer and API.' : 'El punto de partida sin webhook. Consulta el resultado desde el viewer y por API.' }}</p><button @click="tryExample('basic')">{{ ui.replayBasic }}</button><pre aria-label="Ejemplo básico">{{ requestSchemaExample }}</pre></article><article><span class="example-version">V1 · WEBHOOK</span><h2>{{ uiLanguage === 'en' ? 'Terminal result' : 'Resultado terminal' }}</h2><p>{{ uiLanguage === 'en' ? 'The webhook is sent only when the whole execution ends. There is no event per Pokémon.' : 'El webhook sólo se envía cuando termina toda la ejecución. No hay un evento por cada Pokémon.' }}</p><button @click="tryExample('pokemon-v1-webhook')">{{ ui.replayV1 }}</button><pre aria-label="Ejemplo V1 con webhook">{{ pokemonV1WebhookExample }}</pre></article><article><span class="example-version">V2 · WEBHOOK</span><h2>{{ uiLanguage === 'en' ? 'Complete chains' : 'Cadenas completas' }}</h2><p>{{ uiLanguage === 'en' ? 'Specifically designed for V2: each Pokémon is published after its children finish. Change ordering and maxInFlight in the editor.' : 'Ejemplo pensado específicamente para V2: cada Pokémon se publica al completar sus hijos. Cambia ordering y maxInFlight en el editor.' }}</p><button @click="tryExample('pokemon-v2-webhook')">{{ ui.replayV2 }}</button><pre aria-label="Ejemplo V2 con webhook">{{ pokemonV2WebhookExample }}</pre></article></div></section>
    <section v-else class="documentation"><h1>Abriendo playground…</h1></section>
  </main>
</template>

<style>
:root{color-scheme:dark;font-family:Inter,system-ui,sans-serif;background:#080f20;color:#e4ebff}*{box-sizing:border-box}body{margin:0}.shell{min-height:100vh;background:radial-gradient(circle at 85% -20%,#16325a77,transparent 34rem),#080f20}header{height:58px;padding:0 max(18px,calc((100vw - 1280px)/2));display:flex;align-items:center;justify-content:space-between;background:#121b34;border-bottom:1px solid #1a2945}a{color:#c8d2e7;text-decoration:none}.brand{font-weight:800;letter-spacing:1.6px;color:#e4ebff}.brand small{padding:4px 6px;color:#85bff1;background:#1d2944;margin-left:8px}nav{display:flex;align-items:center;gap:18px}button{border:0;border-radius:3px;background:#4cdcb3;color:#071124;padding:11px 15px;font-weight:700;cursor:pointer}nav button{padding:7px 10px;background:#1d2944;color:#e4ebff}.documentation,.login{max-width:1280px;margin:0 auto;padding:54px 24px}.login{max-width:440px;padding-top:72px}.eyebrow{font:700 11px ui-monospace,monospace;letter-spacing:1px;color:#4cdcb3}h1{font-size:clamp(32px,5vw,50px);letter-spacing:-2px;margin:8px 0 10px}h2{margin:0 0 12px}h3{margin:0;font-size:17px}p{line-height:1.65;color:#c4cce0}.lead{max-width:740px}button:focus-visible{outline:2px solid #68e9ff;outline-offset:3px}.tabs{display:flex;flex-wrap:wrap;gap:8px;margin:34px 0 18px;border-bottom:1px solid #263653}.tabs button{background:transparent;color:#9aa8c2;border-radius:0}.tabs button.active{color:#4cdcb3;border-bottom:2px solid #4cdcb3}.configuration,.request-guide{display:grid;gap:20px}.section-heading{display:flex;justify-content:space-between;align-items:end;gap:24px}.section-heading p{margin:0}.section-heading input{max-width:330px}.configuration-sections{display:grid;gap:18px}.configuration-section{background:#0c1528;border:1px solid #1b2943;padding:18px}.configuration-section h3{margin-bottom:15px;color:#e4ebff}.table-wrap{overflow:auto;border:1px solid #263653}.configuration table,.request-table{width:100%;border-collapse:collapse;min-width:900px}.configuration th,.request-table th{background:#121e36;color:#9fafd0;font:700 11px ui-monospace,monospace;letter-spacing:.4px;text-align:left}.configuration th,.configuration td,.request-table th,.request-table td{padding:12px;border-bottom:1px solid #20304d;vertical-align:top}.configuration td:nth-child(1){width:20%}.configuration td:nth-child(2){width:42%;color:#c4cce0}.configuration td:nth-child(3),.configuration td:nth-child(4){width:19%;font:12px ui-monospace,monospace}.request-table td:nth-child(1){width:20%}.request-table td:nth-child(2){width:12%}.request-table td:nth-child(3){width:43%;color:#c4cce0}.request-table td:nth-child(4){width:25%;font:12px ui-monospace,monospace}.requirement{color:#d8c78c;font-size:12px}.current{color:#59e1c0}.muted{color:#9aa8c2}.service-guide{margin-top:30px}.request-intro{display:flex;justify-content:space-between;gap:24px;padding:24px;background:#101a30;border:1px solid #263653}.request-intro p{max-width:760px;margin:8px 0}.schema-link{height:max-content;white-space:nowrap;padding:10px 12px;background:#1d2944;color:#68e9ff;font:700 12px ui-monospace,monospace}.schema-example{display:grid;grid-template-columns:minmax(240px,.75fr) minmax(0,1.25fr);gap:24px;padding:24px;background:#0c1528;border:1px solid #1b2943}.schema-example p{margin:0}.schema-example pre{margin:0;max-height:410px}.doc-layout{display:grid;grid-template-columns:300px minmax(0,1fr);gap:24px}.doc-layout aside{height:max-content;position:sticky;top:20px;padding:18px;background:#101a30;border:1px solid #263653}.doc-layout input,input{width:100%;border:1px solid #263653;background:#071124;color:#e4ebff;padding:12px;border-radius:3px}.doc-layout ul{padding:0;margin:16px 0 0;list-style:none;display:grid;gap:12px}.doc-layout li{display:grid;gap:4px;font-size:12px}.doc-layout li span{color:#9aa8c2}.markdown{min-width:0;padding:8px 24px;background:#0c1528;border:1px solid #1b2943}.markdown :deep(h1),.markdown :deep(h2),.markdown :deep(h3){margin:28px 0 12px;letter-spacing:-.5px}.markdown :deep(p),.markdown :deep(li){color:#c4cce0;line-height:1.7}.markdown :deep(pre),pre{overflow:auto;background:#071124;border:1px solid #263653;padding:16px;color:#68e9ff}.markdown :deep(code),code{color:#68e9ff}.markdown :deep(table){width:100%;border-collapse:collapse;display:block;overflow:auto}.markdown :deep(th),.markdown :deep(td){padding:9px;border:1px solid #263653;text-align:left}.example-grid{display:grid;grid-template-columns:minmax(260px,.8fr) minmax(0,1.2fr);gap:24px;margin-top:28px}.example-grid article{padding:24px;background:#101a30;border:1px solid #263653}.example-grid pre{margin:0;max-height:620px}.examples-grid{grid-template-columns:repeat(3,minmax(0,1fr))}.examples-grid article{display:flex;flex-direction:column;gap:14px;min-width:0;overflow:hidden}.examples-grid article h2,.examples-grid article p{margin:0}.examples-grid article button{align-self:flex-start}.examples-grid pre{width:100%;max-width:100%;max-height:32rem;min-width:0;overflow:auto}@media(max-width:1080px){.examples-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}.error{color:#ff9a8b;margin:0}form{margin-top:30px;display:grid;gap:16px;padding:24px;background:#101a30;border:1px solid #263653}label{display:grid;gap:7px;color:#c8d2e7}@media(max-width:760px){header{padding:0 14px}.brand{font-size:12px}nav{gap:10px;font-size:12px}.documentation{padding:38px 16px}.section-heading,.request-intro{display:grid;align-items:stretch}.section-heading input{max-width:none}.schema-example,.doc-layout,.example-grid{grid-template-columns:1fr}.doc-layout aside{position:static}.markdown{padding:8px 16px}}
</style>
