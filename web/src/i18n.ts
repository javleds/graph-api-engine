export type UILanguage = 'es' | 'en';

export const uiLanguage = (): UILanguage =>
  document.documentElement.dataset.uiLanguage === 'en' ? 'en' : 'es';

const messages = {
  es: {
    docs: 'Docs', examples: 'Ejemplos', playground: 'Playground', logout: 'Cerrar sesión',
    loginEyebrow: 'CONSOLA DE EJECUCIÓN', loginTitle: 'Inicia sesión.', loginLead: 'Accede al playground y a la documentación del motor.', loginButton: 'Entrar al playground',
    docsEyebrow: 'DOCUMENTACIÓN INTERACTIVA', docsTitle: 'Opera y consume API Engine.', docsLead: 'Consulta la guía completa, los defaults y la configuración efectiva de este servicio.',
    service: 'Servicio y arranque', consumption: 'Consumo de grafos', results: 'Obtención de resultados',
    examplesEyebrow: 'EJEMPLOS REPRODUCIBLES', examplesTitle: 'PokéAPI · tres formas de consumir el grafo',
    replayBasic: 'Replay básico →', replayV1: 'Replay V1 →', replayV2: 'Replay V2 →',
  },
  en: {
    docs: 'Docs', examples: 'Examples', playground: 'Playground', logout: 'Sign out',
    loginEyebrow: 'EXECUTION CONSOLE', loginTitle: 'Sign in.', loginLead: 'Access the playground and the engine documentation.', loginButton: 'Enter playground',
    docsEyebrow: 'INTERACTIVE DOCUMENTATION', docsTitle: 'Operate and consume API Engine.', docsLead: 'Review the complete guide, defaults, and effective service configuration.',
    service: 'Service and startup', consumption: 'Graph consumption', results: 'Getting results',
    examplesEyebrow: 'REPLAYABLE EXAMPLES', examplesTitle: 'PokéAPI · three ways to consume the graph',
    replayBasic: 'Replay basic →', replayV1: 'Replay V1 →', replayV2: 'Replay V2 →',
  },
} as const;

export function messagesFor(language: UILanguage = uiLanguage()) {
  return messages[language];
}
