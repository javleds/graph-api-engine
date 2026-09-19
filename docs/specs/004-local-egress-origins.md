# Orígenes abiertos en desarrollo local

## Contexto
El playground local requiere probar APIs/webhooks sin editar allowlists para cada dominio. La validación y ambos transportes aplican hoy listas explícitas.
## Objetivo
Permitir cualquier origen en desarrollo mediante una opción explícita, sin cambiar el default restrictivo de producción.
## Alcance
security.egress.allowAnyOriginForDevelopment, default false; aplica al proveedor y webhook, compilador, CLI/validador y transporte real. Activarla en config/config.json local no versionado.
## Fuera de alcance
Desactivar validación de URL, HTTPS en producción, controles de IP/SSRF, redirects, autenticación, cuotas o límites. Redes privadas siguen requiriendo allowedPrivateCIDRs explícito.
## Comportamiento esperado
Con false, listas obligatorias y comportamiento actual. Con true, permitir listas vacías e ignorar su pertenencia; exigir también allowHTTPForDevelopment=true como marcador de desarrollo existente. Las entradas no vacías deben conservar formato válido. No admitir comodines ni inferir apertura de una lista vacía.
## Decisiones técnicas
Booleano explícito trasladado a ValidationLimits y Policy. Construcción de Policy rechaza apertura sin modo desarrollo, además de config.Validate. Esquemas HTTP/API de requests no cambian.
## Componentes afectados
Configuración, compilador de definiciones, política egress, bootstrap de fetch/webhook y documentación.
## Backend
Los transportes siguen resolviendo y validando todas las IPs antes de conectar. El bypass afecta únicamente pertenencia del origin a la lista.
## Frontend
Sin cambios; Validar JSON refleja la configuración local.
## Persistencia
Sin migraciones ni cambios en jobs. La opción solo pertenece a configuración del operador.
## API
Sin endpoints ni cambios en el JSON de ejecución. Nuevo campo de configuración operativa documentado y false en defaults/ejemplo.
## Validaciones
Rechazar true sin allowHTTPForDevelopment. Mantener privadas/loopback/link-local bloqueadas salvo CIDRs explícitos donde ya se permitían; userinfo y esquemas no HTTP siguen bloqueados.
## Permisos y seguridad
La configuración de producción queda cerrada por defecto. La instancia local se publica solo en 127.0.0.1 según Compose. Abrir orígenes no autoriza IPs privadas automáticamente.
## Casos límite
Webhook en otro dominio, lista vacía con flag false, true sin desarrollo, destino FTP/userinfo, DNS mixto público/privado.
## Estrategia de pruebas
Config y propagación, compilador proveedor/webhook, Policy en modo cerrado/abierto con IPs protegidas, integración real contra proveedor/webhook local no listado y CIDR de prueba explícito.
## Estrategia de validación
make check, build/recrear servicio local; verificar readiness y validador con origen no listado sin llamar APIs externas.
## Estrategia de commits
1. docs: specify open origins for local development
2. feat(egress): allow arbitrary origins explicitly in development
## Archivos estimados a modificar
config/defaults y ejemplo, Config/ValidationLimits/Policy y sus consumidores, pruebas, README y spec normativo de configuración.
## Riesgos
No se deben habilitar las opciones de desarrollo en producción. Un proveedor local exige configurar su CIDR; este cambio resuelve orígenes públicos sin ampliar acceso a la red interna.
## Preguntas abiertas
Ninguna.
## Plan de implementación
Registrar spec, implementar flag de punta a punta con pruebas, documentar y activar únicamente en config local, reconstruir/recrear y verificar servicio.
