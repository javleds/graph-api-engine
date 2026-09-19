# 007 · Actualización visual del Playground

## Objetivo

Convertir `/playground` en un tablero técnico oscuro que permita leer el estado de una ejecución, editar su request, explorar su resultado y revisar sus intercambios HTTP sin perder ninguna capacidad existente.

## Alcance

- Mantener las rutas, contratos, IDs de DOM y controles actuales de v1 y v2.
- Reorganizar la página en una cabecera de producto, resumen de ejecución, métricas, área de trabajo, reanudación y depuración.
- Aplicar una paleta oscura de alto contraste con acentos verde menta, cian, ámbar y rojo según el estado.
- Conservar las dos columnas en escritorio y una sola columna en pantallas pequeñas.
- Mejorar la apariencia de editor, números de línea, árbol JSON, fragmentos v2 y registros de depuración.

## Decisiones de implementación

La página seguirá siendo HTML, CSS y JavaScript estáticos embebidos por el servidor Go. El cambio usa variables CSS y selectores de presentación; no añade dependencias, llamadas de red ni estado de cliente. Los atributos, IDs y textos operativos consumidos por `playground.js` y sus pruebas se mantienen.

La información dinámica seguirá llegando mediante los elementos existentes: `#status`, `#execution-meta`, `#progress`, `#v2-output`, `#response-*` y `#debug`. El CSS representa esos datos como bandas de estado, tarjetas y paneles de consola.

El encabezado de la ejecución usa una distribución de tres franjas: controles junto al título, detalle temporal en una banda independiente y métricas agrupadas por raíces, tráfico y rendimiento. Cada grupo se construye a partir del mismo objeto `progress` que expone la API.

## Validación

Ejecutar `npm run test:playground` para comprobar los recorridos v1, v2, cancelación, reanudación, accesibilidad de teclado y diseño adaptable. Comprobar visualmente `/playground` en escritorio y móvil una vez reconstruido el servicio local.

## Commits

1. `docs: specify playground dashboard refresh`
2. `feat(playground): refresh dashboard visual design`
3. `docs: refine playground execution layout`
4. `feat(playground): group execution dashboard metrics`
