CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    checksum TEXT NOT NULL,
    applied_at INTEGER NOT NULL
);

CREATE TABLE executions (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'completed_with_errors', 'failed')),
    definition_version INTEGER NOT NULL CHECK (definition_version = 1),
    webhook_enabled INTEGER NOT NULL DEFAULT 0 CHECK (webhook_enabled IN (0, 1)),
    request_fingerprint TEXT NOT NULL,
    idempotency_key_hash TEXT,
    snapshot_blob BLOB NOT NULL,
    progress_json TEXT NOT NULL DEFAULT '{}',
    error_json TEXT,
    error_report_json TEXT NOT NULL DEFAULT '{"errors":[],"errorSummary":{"total":0,"returned":0,"truncated":false},"roots":[]}',
    worker_instance_id TEXT,
    version INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    started_at INTEGER,
    finished_at INTEGER,
    result_expires_at INTEGER,
    metadata_expires_at INTEGER NOT NULL,
    CHECK (started_at IS NULL OR started_at >= created_at),
    CHECK (finished_at IS NULL OR finished_at >= created_at)
);

CREATE UNIQUE INDEX executions_owner_idempotency
ON executions(owner_id, idempotency_key_hash)
WHERE idempotency_key_hash IS NOT NULL;

CREATE INDEX executions_owner_created
ON executions(owner_id, created_at DESC);

CREATE INDEX executions_recovery
ON executions(status, worker_instance_id);

CREATE INDEX executions_retention
ON executions(metadata_expires_at);

CREATE TABLE execution_results (
    execution_id TEXT PRIMARY KEY REFERENCES executions(id) ON DELETE CASCADE,
    content_type TEXT NOT NULL DEFAULT 'application/json',
    result_blob BLOB NOT NULL,
    plaintext_size INTEGER NOT NULL CHECK (plaintext_size >= 0),
    sha256 TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);

CREATE INDEX execution_results_expiration
ON execution_results(expires_at);

CREATE TABLE webhook_deliveries (
    id TEXT PRIMARY KEY,
    event_id TEXT NOT NULL UNIQUE,
    execution_id TEXT NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
    owner_id TEXT NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('execution.completed', 'execution.completed_with_errors', 'execution.failed')),
    target_blob BLOB NOT NULL,
    payload_blob BLOB NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'delivering', 'retrying', 'delivered', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at INTEGER NOT NULL,
    lease_owner TEXT,
    lease_expires_at INTEGER,
    version INTEGER NOT NULL DEFAULT 0,
    last_http_status INTEGER,
    last_error_json TEXT,
    created_at INTEGER NOT NULL,
    delivered_at INTEGER,
    deadline_at INTEGER NOT NULL,
    UNIQUE(execution_id, event_type)
);

CREATE INDEX webhook_deliveries_due
ON webhook_deliveries(status, next_attempt_at);

CREATE INDEX webhook_deliveries_lease
ON webhook_deliveries(status, lease_expires_at);

CREATE TABLE quota_cooldowns (
    scope_key TEXT PRIMARY KEY,
    not_before INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX quota_cooldowns_expiration
ON quota_cooldowns(not_before);
