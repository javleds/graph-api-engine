import { createI18n } from 'vue-i18n';

export type UILanguage = 'es' | 'en';

// Strings still sourced by legacy markdown/templates. Keeping them in the
// catalog makes the remaining translation work explicit and prevents new UI
// surfaces from silently escaping the language boundary.
export const cataloguedSpanishStrings = [
  'Del request al resultado.',
  'Construye tu grafo, ejecuta sus llamadas y explora la respuesta completa.',
  '○ Sin ejecución',
  'Listo para ejecutar',
  'Detener seguimiento',
  'Abortar ejecución',
  'Detalle de ejecución',
  'La ejecución aparecerá aquí al enviarla.',
  'MÉTRICAS DE GRAFO',
  'Se actualizan durante la ejecución',
  'Configuración y Grafo',
  'Cargar ejemplo',
  'VERSIÓN DEL MOTOR',
  'v1 · Resultado completo',
  'v2 · Fragmentos progresivos',
  'JSON DE EJECUCIÓN',
  'Se envía el texto original. Configura un destino permitido por el servicio.',
  'IDEMPOTENCY-KEY',
  'OPCIONAL',
  'Ejecutar request',
  'Explorador de Árbol',
  '02 RESPUESTA',
  'El resultado aparecerá cuando termine el job.',
  'Tu respuesta, aquí.',
  'Ejecuta un request o retoma una ejecución por su ID.',
  'RESPUESTA JSON ORIGINAL (SOLO LECTURA)',
  '↻ Retomar una ejecución',
  'Detener el seguimiento no cancela el job.',
  'ID DE EJECUCIÓN',
  'Consultar / reanudar',
  'API Engine · Playground local',
  'Identidad compartida entre visitantes locales',
  'Configuración efectiva',
  'Valores cargados al arrancar. Las credenciales, claves y rutas de secretos no se exponen.',
  'Activa o desactiva la consola web interactiva.',
  'Idioma de los textos de la consola web: es o en.',
  'Propiedad',
  'Para qué sirve',
  'Documento',
  'Ejecuciones',
  'Deduplicación',
  'Almacenamiento',
  'Caché',
  'Seguridad',
  'Mantenimiento',
  'Apagado',
  'Observabilidad',
  'Construye un grafo declarativo.',
  'Contrato v1',
  'Request mínimo válido',
  'Campos del request',
  'Busca una propiedad...',
  'Obligatorio',
  'Valores y regla',
  'Autenticación, caché y webhook',
  'V1 · El grafo en una respuesta final',
  'V2 · Cada cadena completa, sin esperar al resto',
  'Cómo consultar los resultados',
  'Si incluyes webhook en el request',
  'Límites y qué hacer cuando se superan',
  'Orden y concurrencia del receptor',
  'Cadena completa, chunk y webhook: tres tamaños distintos',
] as const;

export const uiLanguage = (): UILanguage =>
  document.documentElement.dataset.uiLanguage === 'en' ? 'en' : 'es';

export const messages = {
  es: {
    common: { definedPerItem: 'Se define por cada elemento', empty: 'vacío' },
    playgroundText: {
      executionWillAppear: 'La ejecución aparecerá aquí al enviarla.', exampleBasicLoaded: 'Ejemplo básico cargado. Sustituye example.com por un proveedor permitido que devuelva JSON.', graphMetrics: 'Métricas de grafo', executionFinished: 'Ejecución terminada', stopTracking: 'Detener seguimiento', abortExecution: 'Abortar ejecución', executionId: 'ID de ejecución', created: 'Creación', started: 'Inicio', finished: 'Fin', elapsed: 'Tiempo transcurrido', lastUpdated: 'Última actualización', rootsAndSubtrees: 'Raíces y subárboles', completeRoots: 'raíces completas', partialRoots: 'raíces parciales', failedRoots: 'raíces fallidas', skippedSubtrees: 'subárboles omitidos', logicalRequests: 'requests lógicos', httpAttempts: 'intentos HTTP', coalescedRequests: 'requests compartidos', retries: 'reintentos', trafficAndHttp: 'Tráfico y HTTP', performanceAndNodes: 'Rendimiento y nodos', cacheHits: 'aciertos de caché', completeItems: 'items completos', nodeErrors: 'errores de nodos', interactive: 'Interactivo', resumeExecution: 'Retomar una ejecución', sensitiveRequestHint: 'Incluye datos sensibles del request solo en esta pestaña. No se guarda información en el navegador.', lastStatus: 'Último estado', response: 'Respuesta', leadDescription: 'Construye tu grafo, ejecuta sus llamadas y explora la respuesta completa.', failed: 'Fallido', complete: 'Completo', cancelled: 'Cancelada', pendingNotifications: 'notificaciones pendientes', trackingPaused: 'Seguimiento pausado', creatingExecution: 'Creando ejecución…', pending: 'Pendiente', running: 'En ejecución', completed: 'Completado', completedWithErrors: 'Completado con errores', refreshWebhook: 'Actualizar webhook'
    },
    docs: 'Docs', examples: 'Ejemplos', playground: 'Playground', logout: 'Cerrar sesión',
    loginEyebrow: 'CONSOLA DE EJECUCIÓN', loginTitle: 'Inicia sesión.', loginLead: 'Accede al playground y a la documentación del motor.', loginButton: 'Entrar al playground',
    docsEyebrow: 'DOCUMENTACIÓN INTERACTIVA', docsTitle: 'Opera y consume API Engine.', docsLead: 'Consulta la guía completa, los defaults y la configuración efectiva de este servicio.',
    service: 'Servicio y arranque', consumption: 'Consumo de grafos', results: 'Obtención de resultados',
    examplesEyebrow: 'EJEMPLOS REPRODUCIBLES', examplesTitle: 'PokéAPI · tres formas de consumir el grafo',
    replayBasic: 'Replay básico →', replayV1: 'Replay V1 →', replayV2: 'Replay V2 →',
    cataloguedSpanishStrings,
  },
  en: {
    common: { definedPerItem: 'Defined per item', empty: 'empty' },
    playgroundText: {
      executionWillAppear: 'The execution will appear here after you send it.', exampleBasicLoaded: 'Basic example loaded. Replace example.com with an allowed provider that returns JSON.', graphMetrics: 'Graph metrics', executionFinished: 'Execution finished', stopTracking: 'Stop tracking', abortExecution: 'Abort execution', executionId: 'Execution ID', created: 'Created', started: 'Started', finished: 'Finished', elapsed: 'Elapsed time', lastUpdated: 'Last updated', rootsAndSubtrees: 'Roots and subtrees', completeRoots: 'complete roots', partialRoots: 'partial roots', failedRoots: 'failed roots', skippedSubtrees: 'skipped subtrees', logicalRequests: 'logical requests', httpAttempts: 'HTTP attempts', coalescedRequests: 'shared requests', retries: 'retries', trafficAndHttp: 'Traffic and HTTP', performanceAndNodes: 'Performance and nodes', cacheHits: 'cache hits', completeItems: 'complete items', nodeErrors: 'node errors', interactive: 'Interactive', resumeExecution: 'Resume an execution', sensitiveRequestHint: 'Include sensitive request data only in this tab. No information is stored in the browser.', lastStatus: 'Latest status', response: 'Response', leadDescription: 'Build your graph, run its calls, and explore the complete response.', failed: 'Failed', complete: 'Complete', cancelled: 'Cancelled', pendingNotifications: 'pending notifications', trackingPaused: 'Tracking paused', creatingExecution: 'Creating execution…', pending: 'Pending', running: 'Running', completed: 'Completed', completedWithErrors: 'Completed with errors', refreshWebhook: 'Refresh webhook'
    },
    docs: 'Docs', examples: 'Examples', playground: 'Playground', logout: 'Sign out',
    loginEyebrow: 'EXECUTION CONSOLE', loginTitle: 'Sign in.', loginLead: 'Access the playground and the engine documentation.', loginButton: 'Enter playground',
    docsEyebrow: 'INTERACTIVE DOCUMENTATION', docsTitle: 'Operate and consume API Engine.', docsLead: 'Review the complete guide, defaults, and effective service configuration.',
    service: 'Service and startup', consumption: 'Graph consumption', results: 'Getting results',
    examplesEyebrow: 'REPLAYABLE EXAMPLES', examplesTitle: 'PokéAPI · three ways to consume the graph',
    replayBasic: 'Replay basic →', replayV1: 'Replay V1 →', replayV2: 'Replay V2 →',
    cataloguedSpanishStrings,
  },
} as const;

export const i18n = createI18n({
  legacy: false,
  locale: uiLanguage(),
  fallbackLocale: 'es',
  messages,
});

export function messagesFor(language: UILanguage = uiLanguage()) {
  return messages[language];
}
