# Cache maintenance

The response cache can be emptied on demand through either API version:

```text
POST /v1/cache/clear
POST /v2/cache/clear
```

Both routes perform the same global operation against the service's SQLite response cache. They require a Bearer principal with the dedicated `cache:clear` permission and have no request body. Authenticated playground sessions can perform the same operation through `POST /playground/api/cache/clear`; the console requests confirmation before sending it.

```sh
curl --request POST "$ENGINE_URL/v1/cache/clear" \
  --header "Authorization: Bearer $ENGINE_TOKEN"
```

A successful request returns `200 OK`:

```json
{"deletedEntries":42}
```

`deletedEntries` is the number of rows removed at the time the delete ran; `0` is a successful no-op. A missing or insufficient credential returns `401` or `403`. Cache-storage failures return `503 STORAGE_UNAVAILABLE`.

Clearing affects all owners because the cache is process-wide, although cache keys themselves are owner-isolated. It deletes only completed stored upstream responses. It does not cancel executions, delete execution results or metadata, alter quota/cooldown state, close HTTP connections, or stop requests already in flight. A request that completes concurrently may populate the cache again after the clear.
