ALTER TABLE v2_state ADD COLUMN results_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE v2_state ADD COLUMN last_result_sequence INTEGER NOT NULL DEFAULT 0;
ALTER TABLE v2_state ADD COLUMN webhook_cursor INTEGER NOT NULL DEFAULT 0;
CREATE INDEX executions_node_webhook ON executions(id) WHERE api_version=2 AND webhook_enabled=1;
CREATE INDEX v2_results_pending_webhooks ON v2_state(execution_id) WHERE results_enabled=1 AND webhook_cursor<last_result_sequence;
CREATE TABLE v2_results (
 execution_id TEXT NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
 node_id TEXT NOT NULL,
 sequence INTEGER NOT NULL,
 event_id TEXT NOT NULL UNIQUE,
 payload_blob BLOB NOT NULL,
 plaintext_size INTEGER NOT NULL,
 first_chunk INTEGER NOT NULL,
 last_chunk INTEGER NOT NULL,
 occurred_at INTEGER NOT NULL,
 PRIMARY KEY(execution_id,node_id),
 UNIQUE(execution_id,sequence)
);
CREATE TRIGGER v2_results_insert AFTER INSERT ON v2_results BEGIN UPDATE v2_state SET stored_bytes=stored_bytes+NEW.plaintext_size WHERE execution_id=NEW.execution_id; END;
CREATE TRIGGER v2_results_delete AFTER DELETE ON v2_results BEGIN UPDATE v2_state SET stored_bytes=stored_bytes-OLD.plaintext_size WHERE execution_id=OLD.execution_id; END;

CREATE TABLE webhook_deliveries_new (
 id TEXT PRIMARY KEY,
 event_id TEXT NOT NULL UNIQUE,
 execution_id TEXT NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
 owner_id TEXT NOT NULL,
 event_type TEXT NOT NULL CHECK(event_type IN ('execution.completed','execution.completed_with_errors','execution.failed','node.finished')),
 target_blob BLOB NOT NULL,
 payload_blob BLOB NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('pending','delivering','retrying','delivered','failed')),
 attempts INTEGER NOT NULL DEFAULT 0 CHECK(attempts>=0),
 next_attempt_at INTEGER NOT NULL,
 lease_owner TEXT,
 lease_expires_at INTEGER,
 version INTEGER NOT NULL DEFAULT 0,
 last_http_status INTEGER,
 last_error_json TEXT,
 created_at INTEGER NOT NULL,
 delivered_at INTEGER,
 deadline_at INTEGER NOT NULL,
 plaintext_size INTEGER NOT NULL DEFAULT 0
);
INSERT INTO webhook_deliveries_new(id,event_id,execution_id,owner_id,event_type,target_blob,payload_blob,status,attempts,next_attempt_at,lease_owner,lease_expires_at,version,last_http_status,last_error_json,created_at,delivered_at,deadline_at)
 SELECT id,event_id,execution_id,owner_id,event_type,target_blob,payload_blob,status,attempts,next_attempt_at,lease_owner,lease_expires_at,version,last_http_status,last_error_json,created_at,delivered_at,deadline_at FROM webhook_deliveries;
DROP TABLE webhook_deliveries;
ALTER TABLE webhook_deliveries_new RENAME TO webhook_deliveries;
CREATE UNIQUE INDEX webhook_terminal_unique ON webhook_deliveries(execution_id,event_type) WHERE event_type<>'node.finished';
CREATE INDEX webhook_deliveries_due ON webhook_deliveries(status,next_attempt_at);
CREATE INDEX webhook_deliveries_lease ON webhook_deliveries(status,lease_expires_at);
CREATE INDEX webhook_deliveries_execution ON webhook_deliveries(execution_id,created_at,id);
CREATE TRIGGER webhook_node_insert AFTER INSERT ON webhook_deliveries WHEN NEW.event_type='node.finished' BEGIN UPDATE v2_state SET stored_bytes=stored_bytes+NEW.plaintext_size WHERE execution_id=NEW.execution_id; END;
CREATE TRIGGER webhook_node_delete AFTER DELETE ON webhook_deliveries WHEN OLD.event_type='node.finished' BEGIN UPDATE v2_state SET stored_bytes=stored_bytes-OLD.plaintext_size WHERE execution_id=OLD.execution_id; END;
