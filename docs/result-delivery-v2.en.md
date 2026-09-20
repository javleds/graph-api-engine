## V2 · Each complete chain, without waiting for the rest

V2 publishes definitive results during execution. A `single` root is ready when it and all children finish; a `collection` root publishes each item with all descendants. Published chains do not change. Independent branches run in parallel within configured concurrency, quotas, and retry limits. Results are published by completion order.

### How to query results

1. Send the request to `POST /v2/executions` and save its `id`.
2. Read `GET /v2/executions/{id}/results?after=0&limit=100`, including while status is `running`.
3. Follow `links.result` or call `/results/{nodeId}`; it returns the complete chain in `data`, diagnostics, and `complete: true`.
4. Save `nextCursor` after processing a page. Continue while `hasMore` is true; when it is false and `terminal` is false, poll again after about one second.

Cursors are decimal strings, not JavaScript numbers. An empty page does not mean execution is finished. Even after `terminal: true`, consume pending pages.

### When the request includes a webhook

Without a webhook, results remain available through the API. With one, new V2 executions emit one `node.finished` event per completed chain, plus the terminal execution event.

```json
{
  "url": "https://consumer.example/events",
  "payload": "status",
  "ordering": "completion",
  "maxInFlight": 1
}
```

V2 accepts only `status` (or an omitted payload). Node events always include the chain in `result`, equivalent to the node API's `data`.

#### Receiver ordering and concurrency

| Setting | Delivery behavior |
| --- | --- |
| `ordering: "completion"`, `maxInFlight: 1` | First completed chain is admitted first; the next waits for a 2xx or terminal failure. |
| `ordering: "completion"`, `maxInFlight: 4` | Up to four deliveries can be unresolved, selected by completion order. |
| `ordering: "source"`, `maxInFlight: 1` | Original request and collection positions are admitted one at a time. |
| `ordering: "completion"`, `maxInFlight: 0` | No per-execution limit; `webhooks.maxConcurrent` still limits the service. |

`maxInFlight` is an integer greater than or equal to zero. Retries retain their place in the window. Webhook ordering does not change engine parallelism or API availability: a fast Charizard result can be read before a slow Bulbasaur even when its webhook waits for source order.

### Chain, chunk, and webhook

A chunk is one piece of the incremental protocol. A result is a finished chain made of many chunks. A webhook is the HTTP delivery attempt for that chain. A chunk remains capped at 256 KiB; `webhooks.maxPayloadBytes` is configurable and includes the envelope. If a webhook is too large, the node remains available through `/results/{nodeId}`.

### Example: language, Bulbasaur, and the next Pokémon

Bulbasaur is published when its `species` call and all descendants finish; it does not wait for other Pokémon. If Charizard finishes first, it is published first while retaining its source index. A failed child can produce `completed_with_errors` with valid data and diagnostics.

### Chunks, downloads, and compatibility

`/chunks` remains available for low-level incremental consumption, `/tree` explores branches, and `/export` downloads an NDJSON snapshot. Results expire with `410`; invalid cursors return `422`. Use the same owner and read permission. Chain delivery applies to new executions with `v2.resultsEnabled: true`; older executions retain their previous terminal webhook behavior.
