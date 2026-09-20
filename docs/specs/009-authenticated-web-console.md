# 009 · Consola web autenticada

## Objetivo

Sustituir el playground abierto por una consola web autenticada, con login, documentación y ejemplos, sin cambiar los contratos autenticados de `/v1` y `/v2`.

## Contrato

- `.env` define `APP_URL`, `APP_PORT`, `PLAYGROUND_USERNAME` y `PLAYGROUND_PASSWORD`; `config.yml` los referencia y Compose los inyecta al proceso.
- La sesión del playground dura ocho horas y se entrega únicamente en una cookie firmada, `HttpOnly` y `SameSite=Strict`. El navegador no recibe ni persiste un Bearer token.
- `POST /playground/api/session`, `GET /playground/api/session` y `DELETE /playground/api/session` crean, consultan y cierran sesión.
- `/playground`, `/docs` y `/examples` requieren sesión. La API pública continúa usando Bearer tokens y owners independientes.
- Los destinos permiten URLs exactas, `https://*.example.com` y `*`. El comodín de subdominio no incluye el apex. El esquema HTTP y los CIDRs privados siguen controlándose explícitamente.

## Interfaz

La SPA Vue sirve login, playground, documentación de operación y consumo, y ejemplos. El botón de un ejemplo navega a `/playground?example=<id>`; el editor carga solamente el ejemplo empaquetado correspondiente. `/examples` publica tres variantes reproducibles: request básico, V1 con webhook terminal y V2 con `ordering`/`maxInFlight`.

Los ejemplos con webhook apuntan a `POST /playground/api/webhook` en el host actual y envían `X-API-Engine-Playground: demo`. El receptor sólo confirma la entrega para que el usuario pueda observar el comportamiento de la cola; no almacena ni procesa datos de negocio. El origen configurado en `http.publicBaseUrl` debe estar incluido en `security.egress.webhookAllowedOrigins` (el `config.example.yml` ya lo muestra con `${APP_URL}`).

## Validación

Cubrir carga de dotenv, patrones de egress, login/cierre/expiración, autorización de rutas, compatibilidad Bearer, navegación y ejemplo precargado. Ejecutar Go, build web, Playwright y Compose.

## Commits

1. `docs: specify authenticated web console`
2. `feat(config): load dotenv and match egress patterns`
3. `feat(auth): protect playground with signed sessions`
4. `feat(web): add vue console navigation and documentation`
5. `test(web): cover authenticated console workflows`
