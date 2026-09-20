# Obtención de resultados en el portal

## Contexto y objetivo

Explicar a consumidores qué esperar de V1 y V2 al consultar resultados y configurar webhooks. Diferenciar el límite de un chunk, el tamaño de una cadena completa y el máximo del mensaje webhook.

## Alcance y comportamiento

Añadir el tab «Obtención de resultados» en /docs, con introducción breve y selector V1/V2 (V1 inicial). Cada versión describe rutas, momento de publicación, estados, webhook opcional, límites y ejemplos Pokémon. Cambiar de versión no crea ejecuciones ni consulta proveedores.

## Decisiones técnicas y componentes

Reutilizar Vue 3, controles HTML nativos y el renderizado Markdown existentes. Extraer el contenido interactivo en un componente y mantener las guías de cada versión como Markdown importado. Usar tabs con teclado y radios nativos; conservar colores, tablas y código del portal, con diseño móvil.

## Backend, persistencia, API y permisos

Sin cambios de contratos, almacenamiento ni permisos. /docs conserva su sesión actual. Los ejemplos de API usan rutas públicas con autenticación del mismo owner. Ajustar la referencia de campos webhook existente para que no contradiga V2.

## Validaciones y casos límite

No confundir definition.version con la versión de API. V1 payload result puede omitirse por tamaño; V2 admite payload status en el request pero entrega automáticamente nodos completos y terminal. Chunk demasiado grande falla procesamiento; mensaje de nodo demasiado grande falla sólo la notificación. Explicitar orden no garantizado, duplicados, expiración y ejecuciones v2 anteriores sin resultados completos.

## Pruebas y validación

Navegador desktop/mobile: acceso a nueva pestaña, cambio V1/V2, contenido/rutas/límites correctos, teclado y navegación entre pestañas, sin crear ejecuciones. Ejecutar typecheck y build web, suite del portal y diff check. Regenerar assets embebidos.

## Plan y estrategia de commits

1. Redactar documentación desde handlers, errores, configuración y payload builders actuales.
2. Implementar pestaña y selector reutilizando el diseño existente.
3. Añadir pruebas de interacción y validar build/typecheck/browser.

Agrupar como funcionalidad documental con pruebas y assets; no crear commits sin una nueva solicitud explícita en esta tarea.

## Archivos estimados, riesgos y preguntas abiertas

App.vue, componente ResultsGuide.vue, guías Markdown de resultados, pruebas del portal y assets compilados. Riesgo principal: describir defaults como límites configurados o mezclar errores de entrega con procesamiento. Mostrar defaults como tales y conservar la pestaña de configuración efectiva como referencia. Ninguna pregunta abierta.
