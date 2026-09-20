# Configurar y arrancar el servicio

Copiar `.env.example` como `.env`. `APP_URL` define la URL pública y `APP_PORT` define el puerto del proceso y de Compose. El archivo contiene las credenciales del playground; no debe versionarse.

`config/config.yml` es la configuración de operador. Usa interpolación `${VARIABLE}` para consumir `.env`. Las listas `security.egress.upstreamAllowedOrigins` y `webhookAllowedOrigins` aceptan URLs exactas, `https://*.example.com` y `*`; HTTP requiere `allowHTTPForDevelopment: true` y las IP privadas siguen restringidas por CIDR.

La sección `ui` controla la consola web completa (login, documentación, ejemplos, playground y sus assets):

```yaml
ui:
  enabled: true       # false devuelve 404 en las rutas de la consola
  language: es        # es o en; se selecciona al iniciar el servicio
```

El idioma predeterminado es español. Las propiedades técnicas, valores booleanos, rutas y bloques de código no se traducen. La API, los workers, health checks y métricas continúan disponibles aunque `ui.enabled` sea `false`.

Para iniciar toda la aplicación:

```sh
docker compose up -d --build
curl http://localhost:8080/health/ready
```

Abrir `/login`, iniciar sesión y continuar hacia `/playground`. El proceso también puede iniciarse directamente con `./bin/api-engine serve --config config/config.yml`.
