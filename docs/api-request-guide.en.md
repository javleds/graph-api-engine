# Build an execution JSON request

An execution request declares a graph: a provider, one or more root calls, and optional child calls that enrich each value. The same graph language is used by V1 and V2; the URL selects the delivery protocol.

## 1. Choose the tree shape

Use `single` when a call returns one object. Use `collection` when a call returns an array or paginated list. Children run once for a single value and once for every collection item. A V2 result is complete only after the node and all of its descendants finish.

## 2. Complete minimal request

```json
{
  "definition": {
    "version": 1,
    "api": {
      "baseUrl": "https://pokeapi.co/api/v2",
      "limits": { "requests": 100, "per": "1m", "concurrency": 8 }
    },
    "calls": [{
      "name": "pokemon",
      "type": "single",
      "request": { "method": "GET", "path": "/pokemon/1" }
    }]
  }
}
```

The configured egress allowlists and request limits also apply to this graph.

## 3. Wrapper and provider fields

| Property | Required | Purpose |
| --- | --- | --- |
| `definition` | Yes | Graph, provider, and root calls. |
| `definition.version` | Yes | Graph language version; currently `1`. |
| `definition.api.baseUrl` | Yes | Absolute HTTPS provider URL. |
| `definition.api.limits` | Yes | Requests, time window, and optional concurrency. |
| `definition.api.auth` | No | `none`, `bearer`, `basic`, or `header` credentials. |
| `definition.api.headers` | No | Static headers applied to provider calls. |
| `definition.api.cache` | No | Inherited cache policy, or `false`. |
| `definition.calls` | Yes | Non-empty array of independent root nodes. |
| `webhook` | No | V1 terminal notification or V2 chain and terminal notifications. |

## 4. Define a node and its request

Each node has a unique sibling `name`, a `type` (`single` or `collection`), a `request`, and optional `select`, `children`, `pagination`, `limits`, and `cache` fields.

```json
{
  "name": "species",
  "type": "single",
  "select": "species",
  "request": { "method": "GET", "path": "/pokemon/{parent.id}" }
}
```

`request.path` starts with `/`, cannot contain a query or fragment, and may use parent or ancestor bindings. Query values may be strings, numbers, booleans, or bindings.

## 5. Selection, contexts, and dynamic parameters

`select` is a dotted path over the remote JSON. Omit it to keep the complete response. A child can read the current parent and its ancestors through bindings. Missing values are reported as node diagnostics rather than silently converted to a successful `null`.

The result preserves the selected value and attaches child properties below it. Collection indexes are absolute, including paginated pages.

## 6. Complete multi-branch example

Use multiple entries in `definition.calls` for independent roots. V2 executes independent branches concurrently and publishes each complete chain by finish order. V1 waits and returns both roots together.

## 7. Page-number pagination

```json
"pagination": {
  "page": "offset",
  "pageSize": "limit",
  "start": 0,
  "pages": "all"
}
```

`page` is the query parameter containing the page number. `pageSize` is optional. `start` defaults to `1`; `pages` accepts `all` or a positive integer. Repeated pages are detected when enabled and stop safely.

## 8. Authentication and headers

Use `bearer`, `basic`, or `header` under `definition.api.auth`. Secrets are never written to the public result. Node request headers override provider headers by name.

## 9. Quotas, concurrency, and cache

Provider limits are requests per time window plus optional concurrency. Global fetch limits and retry settings remain in force. A cache object requires a `ttl`; `false` disables the inherited policy. Node and provider cache policies can override their parent.

## 10. Optional webhook

```json
{
  "url": "https://consumer.example/events",
  "payload": "status",
  "ordering": "completion",
  "maxInFlight": 1
}
```

V1 accepts `status` or `result` and sends one terminal event. V2 accepts `status` (or omission) and sends one `node.finished` event per complete chain plus the terminal event. `ordering` is `completion` by default or `source`; `maxInFlight` defaults to `1`, accepts `0` for no per-execution limit, and is always bounded by the global webhook limit.

## 11. Validation and operational limits

The schema validates shape and types. Runtime limits additionally constrain definition depth, node count, children, root calls, pages, logical requests, HTTP attempts, result bytes, V2 chunks, published bytes, response sizes, and execution time. Invalid URLs, private egress, unsupported methods, and disallowed webhook origins fail before execution.

## 12. Interpret results and errors

V1 exposes the final graph through `/result`. V2 exposes stable complete chains through `/results`, low-level fragments through `/chunks`, branch exploration through `/tree`, and an NDJSON snapshot through `/export`. A node can finish with `completed`, `completed_with_errors`, or `failed`; inspect diagnostics and status instead of treating `null` as success.

## V2 executions

Create V2 executions at `POST /v2/executions`. Poll status and consume result pages using the cursor returned by the server. Store the cursor only after applying a page, deduplicate by `eventId`, and continue reading pending pages even after the terminal status is visible.
