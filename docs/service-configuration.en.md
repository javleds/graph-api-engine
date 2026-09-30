# Configure and start the service

After building, run `./bin/api-engine init` from the repository root. It creates missing local files: `.env` from `.env.example`, `config/config.yml` from `config/config.example.yml`, two independent 32-byte keys, and `secrets/engine-principals.json`. It prints the `local-developer` bearer token only when it creates the principals file; store it in a secret manager. It never stores that token in plaintext and never overwrites existing files, so it is safe to run again.

`APP_URL` sets the public URL and `APP_PORT` sets the process and Compose port. The file contains playground credentials and must not be committed.

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

## Upstream response cache

The operator `cache` section controls **capacity and safety**; it does not enable caching for every request. A consumer decides whether to reuse responses per graph with `definition.api.cache: { ttl: "5m" }` or per node with `calls[].cache`. Each entry contains the encrypted base HTTP response, not the enriched execution result.

`maxEntryBytes` rejects only cache storage for a large response: the execution may still use that response when it meets other limits. `maxEntries` and `maxLogicalBytes` are global budgets; older entries are evicted when either is reached. `maxTTL` is the longest freshness duration a consumer may request, not a default TTL. To invalidate every stored response, a principal with `cache:clear` can call `POST /v1/cache/clear` or `POST /v2/cache/clear`; this affects every consumer of this instance.

## Shared quotas and concurrency

`fetch.maxConcurrentHTTPRequests` is the instance-wide ceiling for upstream requests in flight. It is different from `api.limits`: a request declares the known provider quota and the engine paces every physical attempt, including retries and pagination. A `429` or `Retry-After` publishes a cooldown for the affected scope, so other executions sharing it wait; the cooldown is retained in SQLite across a restart.

Without a managed group, executions with the same upstream origin, credential, and `api.limits` share a quota. Different `api.limits` values have separate scopes. If the provider limits by account, IP, multiple API keys, or multiple origins, declare an operator group so all consumers obey one restriction:

```yaml
throttle:
  quotaGroups:
    - name: primary-provider
      origins: ["https://api.provider.example"]
      credentialMode: shared
      limits:
        requests: 100
        per: "1m"
        concurrency: 4
```

With `credentialMode: shared`, every credential against those origins shares the quota. With `separate` —the default— each credential has its own quota. When a group covers the origin, its limits take precedence over `api.limits` and are shared by the group's executions. `call.limits` still adds endpoint restrictions. Two instances with separate data directories do not share pacing; horizontal scaling needs distributed coordination first.

For controlled cache invalidation, grant selected API principals the `cache:clear` permission and use the documented versioned routes. Authenticated playground sessions can also clear the shared response cache after a confirmation prompt; see [Cache maintenance](cache-maintenance.en.md).
