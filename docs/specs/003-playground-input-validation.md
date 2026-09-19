# Validación del input del playground

## Contexto
El playground ejecuta requests y muestra errores, pero falta una comprobación independiente antes de crear jobs.
## Objetivo
Validar sintaxis y contrato del request con un botón, mostrando éxito/error sin ejecución. Facilitar depuración con números de línea.
## Alcance
Botón Validar JSON, resultado local accesible, endpoint de validación sin efectos, gutter de líneas no seleccionable y sincronizado con el textarea.
## Fuera de alcance
Cambiar el schema, probar proveedores, reservar cuotas, persistir jobs o instalar un editor externo.
## Comportamiento esperado
Enviar el texto original al validador. Mostrar JSON válido para el contrato o error/campo; errores de sintaxis del navegador incluyen línea/columna cuando estén disponibles. Editar, cargar ejemplo o limpiar invalida el diagnóstico anterior y aborta validaciones pendientes. Validar no altera el seguimiento ni su resultado.
## Decisiones técnicas
Reutilizar schema.Parse y mapper.Compile con límites del servidor. Mantener la API autoritativa; no duplicar schema en JavaScript. Gutter aria-hidden, user-select:none y pointer-events:none; textarea sin wrapping para alinear líneas reales. Scroll vertical sincronizado. Sin dependencias nuevas.
## Componentes afectados
Subrouter playground, nuevo handler, assets existentes y pruebas Go/Playwright.
## Backend
Handler acotado por MaxRequestBytes, Content-Type application/json y protección cross-origin existente. Solo parseo y compilación, sin llamar Create, ValidateQuotas, almacenamiento ni transporte.
## Frontend
Acción secundaria junto a Ejecutar. Mensaje propio role=status, colores éxito/error, AbortController independiente y comprobación de generación para evitar diagnósticos obsoletos. Gutter actualizado por input y cambios programáticos.
## Persistencia
Ninguna.
## API
POST /playground/api/validate; 200 {"valid":true}. Errores usan formato existente: 400 JSON inválido, 413 tamaño, 415 media type, 422 schema/semántica. Validación no garantiza disponibilidad de cuotas ni éxito HTTP.
## Validaciones
JSON sintáctico, duplicados, schema, referencias y restricciones configuradas. No requests upstream ni inserciones.
## Permisos y seguridad
Endpoint abierto local, con misma protección cross-origin del playground. No logs del body ni secretos en mensajes añadidos. Números decorativos no accesibles/seleccionables.
## Casos límite
Input vacío, esquema inválido, referencias incorrectas, IDs grandes, red fallida, edición durante validación, documento largo y scroll/resize mobile.
## Estrategia de pruebas
Go valida éxito/rechazos y ausencia de dispatch, límites y cross-origin. Playwright verifica botón sin POST de jobs, diagnóstico invalidado al editar, validación atrasada ignorada, gutter sincronizado y no seleccionable.
## Estrategia de validación
make check, suite navegador, reconstruir Compose local y verificar readiness/playground actualizado.
## Estrategia de commits
1. docs: specify playground input validation
2. feat(playground): validate request JSON and show line numbers
## Archivos estimados a modificar
internal/api/http/{playground_routes.go,validate_request_handler.go}; assets del playground; pruebas Go/Playwright y README.
## Riesgos
Validar no equivale a ejecutar. Long lines requieren scroll horizontal. Los mensajes de syntax de cada navegador pueden variar; mostrar su texto original si no hay ubicación fiable.
## Preguntas abiertas
Ninguna.
## Plan de implementación
Registrar spec, agregar handler puro y pruebas, integrar botón/gutter, probar flujos y actualizar el contenedor local con el código verificado.
