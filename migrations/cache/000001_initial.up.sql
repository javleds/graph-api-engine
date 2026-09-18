CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    checksum TEXT NOT NULL,
    applied_at INTEGER NOT NULL
);

CREATE TABLE http_cache (
    cache_key TEXT PRIMARY KEY,
    schema_version INTEGER NOT NULL,
    response_blob BLOB NOT NULL,
    plaintext_size INTEGER NOT NULL CHECK (plaintext_size >= 0),
    fetched_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    initial_age_ms INTEGER NOT NULL DEFAULT 0 CHECK (initial_age_ms >= 0)
);

CREATE INDEX http_cache_expiration
ON http_cache(expires_at);

CREATE INDEX http_cache_oldest
ON http_cache(fetched_at);
