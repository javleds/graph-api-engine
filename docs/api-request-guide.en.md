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

| Node field | Required | Rule |
| --- | --- | --- |
| `name` | Yes | Output property; 1–64 characters matching `^[A-Za-z_][A-Za-z0-9_]*$`. `parent` is reserved. |
| `type` | Yes | `single` for one non-array value; `collection` for an array. |
| `request` | Yes | Object with `method` and `path`. |
| `select` | No | Dotted path; omitted or `null` selects the complete body. |
| `children` | No | Recursive child nodes with unique sibling names. |
| `pagination` | No | Only for `collection`; see §7. |
| `limits` | No | Extra endpoint restriction; see §9. |
| `cache` | No | Inherits the parent or `api` policy; may override it. |

`request.method` is required and accepts only `"GET"`. `request.path` begins with `/`, never `//`; it cannot contain a complete URL, query, fragment, traversal, or backslashes. There is no request body field.

`request.query` is an object with string, number, or boolean values. It does not accept arrays, objects, or `null`. If an API expects a comma-separated list, send a string such as `"1,2,3"`; V1 has no repeated query parameters.

```json
{
  "method": "GET",
  "path": "/companies/{parent.id}/users",
  "query": { "active": true, "perPage": 50, "region": "{parent.region}" },
  "headers": { "Accept-Language": "en" }
}
```

This is the `request` of a child whose parent has `id` and `region`. Node headers replace `api.headers` by name, without case sensitivity. Headers are not inherited from a node to its children.

## 5. Selection, contexts, and dynamic parameters

### Select a response

| `select` | Meaning |
| --- | --- |
| Omitted or `null` | Complete JSON body. |
| `data.items` | The `items` property inside `data`. |
| `data.0.id` | `id` of the first item in the `data` array. |

For an object, a numeric segment looks up a property with that name. For an array, it selects a zero-based index. This is not JSONPath: do not use `$`, `*`, `[]`, filters, functions, or empty segments. Keys containing a literal dot cannot be selected with this syntax. A missing field produces `SELECT_PATH_NOT_FOUND`; an existing `null` remains `null`, subject to the node type.

`select` runs before pagination and enrichment. It is not a field mapper and does not rename output properties.

### Read parents and ancestors

Bindings are allowed **only in `request.path` and string values in `request.query`**:

| Binding | Reads |
| --- | --- |
| `{parent.id}` | `id` from the immediate parent instance. |
| `{parent.value}` | Normalized scalar parent value. |
| `{companies.id}` | `id` from the `companies` ancestor in this branch. |
| `{parent.address.country}` | Nested property from the parent. |

In a collection, `parent` is each selected item, not the whole array or original response body. Contexts are frozen before enrichment: a child cannot read values attached by a sibling. Roots have no `parent`. A node cannot read itself, a sibling, or another root.

Repeated names in separate branches are valid. If two ancestors in the same branch have the same name, a named reference is ambiguous and rejected; `parent` still identifies the immediate parent.

Resolved values must be strings, numbers, or booleans. A missing property produces `BINDING_PATH_NOT_FOUND`; an object, array, or `null` cannot become a parameter. The engine escapes path and query values separately, so pass source data unescaped. Numbers retain precision, but an ID that JavaScript already rounded cannot be recovered; send large IDs as strings when needed.

### Enriched result shape

- An object is copied and receives one property for each child, using that child's `name`.
- A scalar with children becomes `{ "value": <scalar> }` before it is enriched.
- Values without children remain unchanged.
- `null` and nested arrays cannot be enriched: the engine records `PARENT_NOT_ENRICHABLE`, preserves the position as `null`, and skips their children.
- A child name that collides with an existing object property fails that instance with `OUTPUT_FIELD_COLLISION`; it never overwrites source data.
- Collections preserve source positions and order even when branches finish at different times.

For example, a `productIds` node receives `[10, 20]` and has a `details` child that calls `/products/{parent.value}`. Its enriched value is:

```json
[
  { "value": 10, "details": { "id": 10, "title": "A" } },
  { "value": 20, "details": { "id": 20, "title": "B" } }
]
```

## 6. Complete multi-branch example

Use multiple entries in `definition.calls` for independent roots. V2 executes independent branches concurrently and publishes each complete chain by finish order. V1 waits and returns both roots together.

## 7. Page-number pagination

```json
{
  "name": "users",
  "type": "collection",
  "select": "data.items",
  "request": { "method": "GET", "path": "/users", "query": { "perPage": 50 } },
  "pagination": { "page": "page", "pageSize": "perPage", "start": 1, "pages": "all" }
}
```

| Field | Rule / default |
| --- | --- |
| `page` | Required query-parameter name containing the page number. |
| `pageSize` | Optional query-parameter name containing the requested page size. |
| `start` | Integer ≥ 0; default `1`. |
| `pages` | `"all"` or a positive integer; default `"all"`. It is a maximum page count, not a final page number. |

`page` cannot also appear in `request.query`. When `pageSize` is declared, its query value must be a static positive JSON integer: `50`, not `"50"` or `"{parent.size}"`. The names must differ.

Pagination ends normally on an empty page, a page shorter than the expected size, or after `pages` pages. Without `pageSize`, the first non-empty page establishes the expected size. Without `pagination`, only one page is requested.

A definitive error retains accepted pages and stops that paginator. A page larger than the expected size is discarded with `PAGINATION_SIZE_INCONSISTENT`. A child failure does not stop its parent paginator. Every parent instance has separate counters.

`"all"` remains subject to service limits. It does not follow cursors, offsets, totals, or links returned by the provider. If the provider returns a short page early, this strategy cannot guarantee that every remote item was recovered.

## 8. Authentication and headers

Use one of these forms in `api.auth`:

| Type | Object |
| --- | --- |
| None | `{ "type": "none" }` |
| Bearer | `{ "type": "bearer", "token": "provider-token" }` |
| Basic | `{ "type": "basic", "username": "user", "password": "secret" }` |
| Header | `{ "type": "header", "name": "X-API-Key", "value": "secret" }` |

Omitting `auth` is equivalent to `none`. There is no node-level authentication, OAuth refresh, or provider HMAC signing. Do not manually declare the header controlled by `auth`, for example `Authorization` with bearer or basic.

Headers are static. `${API_TOKEN}` is a literal string: the engine does not expand environment variables in execution JSON and does not interpolate ancestors in headers. Resolve credentials in the client before submitting a request.

Do not set `Host`, `Connection`, `Content-Length`, `Transfer-Encoding`, `TE`, `Trailer`, `Upgrade`, `Keep-Alive`, `Proxy-Authenticate`, `Proxy-Authorization`, or `User-Agent`. Do not repeat names with different casing, and do not include controls or line breaks. The engine adds `Accept: application/json` when it is absent and never follows provider redirects.

## 9. Quotas, concurrency, and cache

```json
{ "requests": 100, "per": "1m", "concurrency": 4 }
```

`api.limits` requires a positive integer `requests` and `per`. `concurrency` is optional; when omitted, the operator's global fetch concurrency applies. Quotas apply to physical attempts, including retries, through pacing. They do not promise a burst of 100 simultaneous requests.

`call.limits` may contain `requests` plus `per`, only `concurrency`, or both. It cannot be empty. It adds an endpoint restriction for every page and ID resolved by that call; it never replaces the provider quota and does not automatically limit children that call other endpoints. Quotas are shared by provider and credential; incompatible declarations for the same scope fail with `QUOTA_POLICY_CONFLICT`.

Durations use one positive integer and unit: `250ms`, `1s`, `5m`, `2h`, or `7d`. Do not use `0s`, `1.5s`, `1h30m`, negative values, or spaces. `1d` means 24 hours.

| `cache` value | Effect |
| --- | --- |
| Omitted in `api` | Caching is disabled. |
| Omitted in a node | Inherits from its parent, or from `api` for a root. |
| `false` | Does not read or write cache for that node; descendants inherit this policy unless they override it. |
| `{ "ttl": "5m" }` | Enables caching with that response-freshness requirement. |

`true`, `null`, and `{}` are not valid cache policies. The engine caches the base HTTP response, not the enriched graph; each child evaluates its own policy. Provider headers and operational limits may prevent a response from being stored or reused. Deduplication of identical simultaneous requests can remain active even when `cache` is `false`.

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
