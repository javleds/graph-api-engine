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
