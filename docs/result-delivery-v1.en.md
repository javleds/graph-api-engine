## V1 · The graph in one final response

V1 assembles the complete result before publishing it. While execution is running you can inspect status and progress, but the data is not downloadable yet.

### How to query results

1. Send the request to `POST /v1/executions`; the `202` response contains an `id` and follow-up links.
2. Poll `GET /v1/executions/{id}` while the status is `pending` or `running`.
3. When it is `completed` or `completed_with_errors`, call `GET /v1/executions/{id}/result`. The `result` property contains the graph; `errors`, `errorSummary`, and `roots` explain partial results.
4. When it is `failed`, read the diagnostic from the status endpoint. The result endpoint returns `409 EXECUTION_FAILED`.

There is no result cursor or chunk endpoint in V1. Calling `/result` too early returns `409 RESULT_NOT_READY`; an expired result returns `410 RESULT_EXPIRED`.

### When the request includes a webhook

The webhook is optional. If omitted or `null`, query the API and no notification is sent. When present, one event is emitted when execution finishes, including executions that finish with errors. V1 does not send one event per Pokémon.

```json
{
  "url": "https://consumer.example/events",
  "payload": "result"
}
```

| Setting | What your endpoint receives |
| --- | --- |
| `payload: "status"`, or omitted | Terminal status, diagnostics, and links; no graph data. |
| `payload: "result"` | The final graph when execution succeeds and the message fits the limit. Check `resultIncluded`. |

The event type is `execution.completed`, `execution.completed_with_errors`, or `execution.failed`. Delivery failure does not change processing status. Retries may duplicate notifications; deduplicate by `eventId`.

### Limits and recovery

| Situation | Behavior | Consumer action |
| --- | --- | --- |
| The graph exceeds `executions.maxResultBytes` | Execution fails with `RESULT_TOO_LARGE`; no final result can be downloaded. | Reduce the query or use V2 for progressive consumption. |
| A result webhook exceeds `webhooks.maxPayloadBytes` | The graph is omitted and `resultIncluded` is `false`. | Download the graph from `/v1/executions/{id}/result` while retained. |
| The receiver rejects or does not answer | Eligible retries run until attempts or delivery age are exhausted. | Inspect status and recover through the API. |

The API remains the source of truth for large results. A graph can fit the V1 result limit while exceeding the webhook limit.

### Example: language and Pokémon species

With a `language` root and a `pokemon` collection, each enriched with `species`, V1 waits for the entire execution. `/result` then returns both roots; neither an early `language` result nor Bulbasaur is published separately.

### Access and retention

Use a token from the execution owner with read permission; a link does not grant anonymous access. Playground routes use its session. Result retention is configurable (seven days after completion by default), while metadata may live longer.
