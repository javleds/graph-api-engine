<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import MarkdownIt from 'markdown-it';
import serviceGuide from '../../docs/service-configuration.md?raw';
import requestGuide from '../../docs/api-request-guide.md?raw';
import v2Guide from '../../docs/api-v2-guide.md?raw';
import pokemonExample from '../../docs/request_examples/poke_api.json?raw';

type Config = Record<string, unknown>;
type Row = { path: string; description: string; defaultValue: unknown; currentValue: unknown };
const markdown = new MarkdownIt({ html: false, linkify: true, typographer: true });
const path = ref(location.pathname);
const username = ref(''), password = ref(''), error = ref(''), loading = ref(false);
const docsTab = ref<'service' | 'consumption'>('service');
const configQuery = ref(''), requestQuery = ref(''), configError = ref('');
const effectiveConfig = ref<{ current: Config; defaults: Config }>();

const sections = [
  ['root', 'Documento'], ['http', 'HTTP'], ['executions', 'Ejecuciones'], ['fetch', 'Fetch'],
  ['throttle', 'Throttle'], ['deduplication', 'Deduplicación'], ['storage', 'Almacenamiento'],
  ['cache', 'Caché'], ['webhooks', 'Webhooks'], ['security', 'Seguridad'], ['maintenance', 'Mantenimiento'],
  ['shutdown', 'Apagado'], ['observability', 'Observabilidad'],
] as const;
const fields: Array<[string, string]> = [
  ['version', 'Versión del formato de configuración que interpreta el servicio.'],
  ['mode', 'Modo operativo y conjunto de drivers admitido por esta versión.'],
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
  ['fetch.maxConcurrentHTTPRequests', 'Límite global de conexiones HTTP salientes simultáneas.'],
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
  ['throttle.utilization', 'Proporción de la cuota declarada que el motor puede consumir.'],
  ['throttle.maxQuotaScopes', 'Máximo de ámbitos de cuota que el motor mantiene en memoria.'],
  ['throttle.coldStart', 'Política aplicada cuando aún no hay una ventana de cuota conocida.'],
  ['throttle.safetyMargin', 'Margen temporal que protege contra exceder una ventana de cuota.'],
  ['throttle.quotaGroups', 'Grupos de cuota compartidos entre orígenes.'],
  ['throttle.quotaGroups[].name', 'Nombre único que identifica el grupo de cuota.'],
  ['throttle.quotaGroups[].origins', 'Orígenes HTTPS que comparten la misma cuota.'],
  ['throttle.quotaGroups[].credentialMode', 'Modo de credenciales: separate o shared para el grupo.'],
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
  ['cache.driver', 'Driver de la caché persistente de respuestas.'],
  ['cache.path', 'Ruta del archivo de base de datos de caché.'],
  ['cache.busyTimeout', 'Tiempo que SQLite espera antes de devolver una caché ocupada.'],
  ['cache.maxOpenConnections', 'Máximo de conexiones abiertas a la base de caché.'],
  ['cache.synchronous', 'Nivel de sincronización de SQLite para la caché.'],
  ['cache.maxEntryBytes', 'Tamaño máximo de una entrada individual de caché.'],
  ['cache.maxEntries', 'Máximo de entradas que puede contener la caché.'],
  ['cache.maxLogicalBytes', 'Presupuesto lógico máximo en bytes para todas las entradas.'],
  ['cache.maxTTL', 'Tiempo de vida máximo que puede solicitar una política de caché.'],
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
const requestFields = [['definition.version', 'Versión del lenguaje de grafos: 1.'], ['definition.api.baseUrl', 'URL base común del proveedor.'], ['definition.api.limits', 'Cuota declarada: requests, per y concurrency opcional.'], ['definition.calls', 'Un nodo raíz o un arreglo de raíces independientes.'], ['calls[].name', 'Nombre de salida único entre hermanos.'], ['calls[].type', 'single para recurso; collection para lista.'], ['calls[].select', 'Selector por puntos sobre el JSON de respuesta.'], ['calls[].request.path', 'Path GET relativo; admite bindings de parent y ancestros.'], ['calls[].children', 'Relaciones que enriquecen cada valor del nodo.'], ['webhook', 'Notificación terminal opcional.']];
function valueAt(source: unknown, field: string): unknown {
  if (field.includes('[]')) return undefined;
  return field.split('.').reduce<unknown>((value, key) => value && typeof value === 'object' && !Array.isArray(value) ? (value as Config)[key] : undefined, source);
}
function display(value: unknown, perItem = false): string {
  if (perItem) return 'Se define por cada elemento';
  if (value === undefined) return '—';
  return typeof value === 'string' ? (value || 'vacío') : JSON.stringify(value);
}
function fieldSection(field: string) { return field.includes('.') ? field.split('.')[0] : 'root'; }
const configRows = computed<Row[]>(() => fields.map(([field, description]) => ({ path: field, description, defaultValue: valueAt(effectiveConfig.value?.defaults, field), currentValue: valueAt(effectiveConfig.value?.current, field) })));
const visibleSections = computed(() => {
  const query = configQuery.value.trim().toLowerCase();
  return sections.map(([id, title]) => ({ id, title, rows: configRows.value.filter((row) => fieldSection(row.path) === id && (!query || `${row.path} ${row.description} ${display(row.currentValue)}`.toLowerCase().includes(query))) })).filter((section) => section.rows.length);
});
function matches(items: string[][], query: string) { const normalized = query.trim().toLowerCase(); return normalized ? items.filter(([name, description]) => `${name} ${description}`.toLowerCase().includes(normalized)) : items; }
const filteredRequestFields = computed(() => matches(requestFields, requestQuery.value));
const renderedService = computed(() => markdown.render(serviceGuide));
const renderedConsumption = computed(() => markdown.render(`${requestGuide}\n\n---\n\n${v2Guide}`));
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
function tryExample() { location.assign('/playground?example=pokemon'); }
onMounted(async () => { if (path.value !== '/login' && !(await fetch('/playground/api/session')).ok) { location.replace('/login'); return; } if (path.value === '/docs') await loadConfiguration(); });
</script>

<template>
  <main class="shell">
    <header><a href="/playground" class="brand">↳ API ENGINE <small>v2</small></a><nav v-if="path !== '/login'"><a href="/playground">Playground</a><a href="/docs">Docs</a><a href="/examples">Ejemplos</a><button @click="logout">Cerrar sesión</button></nav></header>
    <section v-if="path === '/login'" class="login"><p class="eyebrow">CONSOLA DE EJECUCIÓN</p><h1>Inicia sesión.</h1><p>Accede al playground y a la documentación del motor.</p><form @submit.prevent="login"><label>Usuario<input v-model="username" autocomplete="username" required></label><label>Contraseña<input v-model="password" type="password" autocomplete="current-password" required></label><p v-if="error" class="error" role="alert">{{ error }}</p><button :disabled="loading">{{ loading ? 'Validando…' : 'Entrar al playground' }}</button></form></section>
    <section v-else-if="path === '/docs'" class="documentation"><p class="eyebrow">DOCUMENTACIÓN INTERACTIVA</p><h1>Opera y consume API Engine.</h1><p class="lead">Consulta la guía completa, los defaults y la configuración efectiva de este servicio.</p><div class="tabs" role="tablist"><button :class="{ active: docsTab === 'service' }" @click="docsTab = 'service'">Servicio y arranque</button><button :class="{ active: docsTab === 'consumption' }" @click="docsTab = 'consumption'">Consumo de grafos</button></div>
      <template v-if="docsTab === 'service'"><section class="configuration"><div class="section-heading"><div><h2>Configuración efectiva</h2><p>Valores cargados al arrancar. Las credenciales, claves y rutas de secretos no se exponen.</p></div><input v-model="configQuery" placeholder="Buscar propiedad o valor…" aria-label="Buscar propiedad de configuración"></div><p v-if="configError" class="error">{{ configError }}</p><p v-else-if="!effectiveConfig" class="muted">Cargando configuración…</p><div v-else class="configuration-sections"><section v-for="section in visibleSections" :key="section.id" class="configuration-section"><h3>{{ section.title }}</h3><div class="table-wrap"><table><thead><tr><th>Propiedad</th><th>Para qué sirve</th><th>Default</th><th>Valor actual</th></tr></thead><tbody><tr v-for="row in section.rows" :key="row.path"><td><code>{{ row.path }}</code></td><td>{{ row.description }}</td><td><code>{{ display(row.defaultValue, row.path.includes('[]')) }}</code></td><td><code class="current">{{ display(row.currentValue, row.path.includes('[]')) }}</code></td></tr></tbody></table></div></section></div></section><article class="markdown service-guide" v-html="renderedService"></article></template>
      <div v-else class="doc-layout"><aside><h2>Request JSON</h2><input v-model="requestQuery" placeholder="Buscar parámetro…" aria-label="Buscar parámetro de request"><ul><li v-for="([name, description]) in filteredRequestFields" :key="name"><code>{{ name }}</code><span>{{ description }}</span></li></ul></aside><article class="markdown" v-html="renderedConsumption"></article></div>
    </section>
    <section v-else-if="path === '/examples'" class="documentation"><p class="eyebrow">EJEMPLOS</p><h1>PokéAPI · bosque con relaciones</h1><p class="lead">Obtiene el idioma español, los Pokémon de la Pokédex de Paldea y la especie de cada entrada.</p><div class="example-grid"><article><h2>Cómo leerlo</h2><p>Dos raíces independientes. <code>pokemon</code> selecciona <code>pokemon_entries</code>; su hijo usa <code>{parent.pokemon_species.name}</code> para resolver cada especie.</p><button @click="tryExample">Inténtalo en playground →</button></article><pre aria-label="Ejemplo JSON de PokéAPI">{{ pokemonExample }}</pre></div></section>
    <section v-else class="documentation"><h1>Abriendo playground…</h1></section>
  </main>
</template>

<style>
:root{color-scheme:dark;font-family:Inter,system-ui,sans-serif;background:#080f20;color:#e4ebff}*{box-sizing:border-box}body{margin:0}.shell{min-height:100vh;background:radial-gradient(circle at 85% -20%,#16325a77,transparent 34rem),#080f20}header{height:58px;padding:0 max(18px,calc((100vw - 1280px)/2));display:flex;align-items:center;justify-content:space-between;background:#121b34;border-bottom:1px solid #1a2945}a{color:#c8d2e7;text-decoration:none}.brand{font-weight:800;letter-spacing:1.6px;color:#e4ebff}.brand small{padding:4px 6px;color:#85bff1;background:#1d2944;margin-left:8px}nav{display:flex;align-items:center;gap:18px}button{border:0;border-radius:3px;background:#4cdcb3;color:#071124;padding:11px 15px;font-weight:700;cursor:pointer}nav button{padding:7px 10px;background:#1d2944;color:#e4ebff}.documentation,.login{max-width:1280px;margin:0 auto;padding:54px 24px}.login{max-width:440px;padding-top:72px}.eyebrow{font:700 11px ui-monospace,monospace;letter-spacing:1px;color:#4cdcb3}h1{font-size:clamp(32px,5vw,50px);letter-spacing:-2px;margin:8px 0 10px}h2{margin:0 0 12px}h3{margin:0;font-size:17px}p{line-height:1.65;color:#c4cce0}.lead{max-width:740px}.tabs{display:flex;gap:8px;margin:34px 0 18px;border-bottom:1px solid #263653}.tabs button{background:transparent;color:#9aa8c2;border-radius:0}.tabs button.active{color:#4cdcb3;border-bottom:2px solid #4cdcb3}.configuration{display:grid;gap:20px}.section-heading{display:flex;justify-content:space-between;align-items:end;gap:24px}.section-heading p{margin:0}.section-heading input{max-width:330px}.configuration-sections{display:grid;gap:18px}.configuration-section{background:#0c1528;border:1px solid #1b2943;padding:18px}.configuration-section h3{margin-bottom:15px;color:#e4ebff}.table-wrap{overflow:auto;border:1px solid #263653}.configuration table{width:100%;border-collapse:collapse;min-width:900px}.configuration th{background:#121e36;color:#9fafd0;font:700 11px ui-monospace,monospace;letter-spacing:.4px;text-align:left}.configuration th,.configuration td{padding:12px;border-bottom:1px solid #20304d;vertical-align:top}.configuration td:nth-child(1){width:20%}.configuration td:nth-child(2){width:42%;color:#c4cce0}.configuration td:nth-child(3),.configuration td:nth-child(4){width:19%;font:12px ui-monospace,monospace}.current{color:#59e1c0}.muted{color:#9aa8c2}.service-guide{margin-top:30px}.doc-layout{display:grid;grid-template-columns:300px minmax(0,1fr);gap:24px}.doc-layout aside{height:max-content;position:sticky;top:20px;padding:18px;background:#101a30;border:1px solid #263653}.doc-layout input,input{width:100%;border:1px solid #263653;background:#071124;color:#e4ebff;padding:12px;border-radius:3px}.doc-layout ul{padding:0;margin:16px 0 0;list-style:none;display:grid;gap:12px}.doc-layout li{display:grid;gap:4px;font-size:12px}.doc-layout li span{color:#9aa8c2}.markdown{min-width:0;padding:8px 24px;background:#0c1528;border:1px solid #1b2943}.markdown :deep(h1),.markdown :deep(h2),.markdown :deep(h3){margin:28px 0 12px;letter-spacing:-.5px}.markdown :deep(p),.markdown :deep(li){color:#c4cce0;line-height:1.7}.markdown :deep(pre),pre{overflow:auto;background:#071124;border:1px solid #263653;padding:16px;color:#68e9ff}.markdown :deep(code),code{color:#68e9ff}.markdown :deep(table){width:100%;border-collapse:collapse;display:block;overflow:auto}.markdown :deep(th),.markdown :deep(td){padding:9px;border:1px solid #263653;text-align:left}.example-grid{display:grid;grid-template-columns:minmax(260px,.8fr) minmax(0,1.2fr);gap:24px;margin-top:28px}.example-grid article{padding:24px;background:#101a30;border:1px solid #263653}.example-grid pre{margin:0;max-height:620px}.error{color:#ff9a8b;margin:0}form{margin-top:30px;display:grid;gap:16px;padding:24px;background:#101a30;border:1px solid #263653}label{display:grid;gap:7px;color:#c8d2e7}@media(max-width:760px){header{padding:0 14px}.brand{font-size:12px}nav{gap:10px;font-size:12px}.documentation{padding:38px 16px}.section-heading{display:grid;align-items:stretch}.section-heading input{max-width:none}.doc-layout,.example-grid{grid-template-columns:1fr}.doc-layout aside{position:static}.markdown{padding:8px 16px}}
</style>
