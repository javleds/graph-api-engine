# Configurar y arrancar el servicio

Copiar `.env.example` como `.env`. `APP_URL` define la URL pública y `APP_PORT` define el puerto del proceso y de Compose. El archivo contiene las credenciales del playground; no debe versionarse.

`config/config.yml` es la configuración de operador. Usa interpolación `${VARIABLE}` para consumir `.env`. Las listas `security.egress.upstreamAllowedOrigins` y `webhookAllowedOrigins` aceptan URLs exactas, `https://*.example.com` y `*`; HTTP requiere `allowHTTPForDevelopment: true` y las IP privadas siguen restringidas por CIDR.

Para iniciar toda la aplicación:

```sh
docker compose up -d --build
curl http://localhost:8080/health/ready
```

Abrir `/login`, iniciar sesión y continuar hacia `/playground`. El proceso también puede iniciarse directamente con `./bin/api-engine serve --config config/config.yml`.
