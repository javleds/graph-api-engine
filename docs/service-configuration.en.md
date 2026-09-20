# Configure and start the service

Copy `.env.example` to `.env`. `APP_URL` sets the public URL and `APP_PORT` sets the process and Compose port. The file contains playground credentials and must not be committed.

`config/config.yml` is the operator configuration. Use `${VARIABLE}` interpolation to consume `.env`. Egress allowlists accept exact URLs, `https://*.example.com`, and `*`; HTTP requires `allowHTTPForDevelopment: true`, and private IPs remain restricted by CIDR.

The `ui` section controls the complete web console (login, documentation, examples, playground, and assets):

```yaml
ui:
  enabled: true       # false returns 404 for console routes
  language: en        # es or en; selected when the service starts
```

Spanish is the default language. Technical properties, boolean values, routes, and code blocks are not translated. The API, workers, health checks, and metrics remain available when `ui.enabled` is `false`.

```sh
docker compose up -d --build
curl http://localhost:8080/health/ready
```

Open `/login`, sign in, and continue to `/playground`. The process can also be started directly with `./bin/api-engine serve --config config/config.yml`.
