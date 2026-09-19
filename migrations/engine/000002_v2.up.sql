ALTER TABLE executions ADD COLUMN api_version INTEGER NOT NULL DEFAULT 1 CHECK(api_version IN (1,2));
CREATE TABLE v2_state (
 execution_id TEXT PRIMARY KEY REFERENCES executions(id) ON DELETE CASCADE,
 checkpoint_version INTEGER NOT NULL DEFAULT 1,
 deadline_at INTEGER NOT NULL,
 generation INTEGER NOT NULL DEFAULT 1,
 recoveries INTEGER NOT NULL DEFAULT 0,
 last_sequence INTEGER NOT NULL DEFAULT 0,
 published_bytes INTEGER NOT NULL DEFAULT 0,
 stored_bytes INTEGER NOT NULL DEFAULT 0,
 logical_requests INTEGER NOT NULL DEFAULT 0,
 http_attempts INTEGER NOT NULL DEFAULT 0,
 pending_tasks INTEGER NOT NULL DEFAULT 0,
 node_count INTEGER NOT NULL DEFAULT 0,
 terminal INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE v2_tasks (
 execution_id TEXT NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
 task_key TEXT NOT NULL,
 parent_key TEXT NOT NULL DEFAULT '',
 priority INTEGER NOT NULL,
 revision INTEGER NOT NULL DEFAULT 0,
 done INTEGER NOT NULL DEFAULT 0,
 payload_blob BLOB NOT NULL,
 plaintext_size INTEGER NOT NULL,
 PRIMARY KEY(execution_id,task_key)
);
CREATE INDEX v2_tasks_ready ON v2_tasks(execution_id,done,priority,task_key);
CREATE TABLE v2_chunks (
 execution_id TEXT NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
 sequence INTEGER NOT NULL,
 payload_blob BLOB NOT NULL,
 plaintext_size INTEGER NOT NULL,
 PRIMARY KEY(execution_id,sequence)
);
CREATE TABLE v2_nodes (
 execution_id TEXT NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
 node_id TEXT NOT NULL,
 parent_id TEXT NOT NULL,
 position INTEGER NOT NULL,
 payload_blob BLOB NOT NULL,
 plaintext_size INTEGER NOT NULL,
 PRIMARY KEY(execution_id,node_id)
);
CREATE INDEX v2_nodes_children ON v2_nodes(execution_id,parent_id,position,node_id);

CREATE TABLE v2_inputs (
 execution_id TEXT NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
 input_key TEXT NOT NULL,
 payload_blob BLOB NOT NULL,
 plaintext_size INTEGER NOT NULL,
 PRIMARY KEY(execution_id,input_key)
);

CREATE TRIGGER v2_tasks_insert AFTER INSERT ON v2_tasks BEGIN UPDATE v2_state SET stored_bytes=stored_bytes+NEW.plaintext_size,pending_tasks=pending_tasks+(1-NEW.done) WHERE execution_id=NEW.execution_id; END;

CREATE TRIGGER v2_tasks_delete AFTER DELETE ON v2_tasks BEGIN UPDATE v2_state SET stored_bytes=stored_bytes-OLD.plaintext_size,pending_tasks=pending_tasks-(1-OLD.done) WHERE execution_id=OLD.execution_id; END;

CREATE TRIGGER v2_tasks_update AFTER UPDATE ON v2_tasks BEGIN UPDATE v2_state SET stored_bytes=stored_bytes+NEW.plaintext_size-OLD.plaintext_size,pending_tasks=pending_tasks+OLD.done-NEW.done WHERE execution_id=NEW.execution_id; END;

CREATE TRIGGER v2_chunks_insert AFTER INSERT ON v2_chunks BEGIN UPDATE v2_state SET stored_bytes=stored_bytes+NEW.plaintext_size WHERE execution_id=NEW.execution_id; END;

CREATE TRIGGER v2_chunks_delete AFTER DELETE ON v2_chunks BEGIN UPDATE v2_state SET stored_bytes=stored_bytes-OLD.plaintext_size WHERE execution_id=OLD.execution_id; END;

CREATE TRIGGER v2_chunks_update AFTER UPDATE ON v2_chunks BEGIN UPDATE v2_state SET stored_bytes=stored_bytes+NEW.plaintext_size-OLD.plaintext_size WHERE execution_id=NEW.execution_id; END;

CREATE TRIGGER v2_nodes_insert AFTER INSERT ON v2_nodes BEGIN UPDATE v2_state SET stored_bytes=stored_bytes+NEW.plaintext_size,node_count=node_count +1 WHERE execution_id=NEW.execution_id; END;

CREATE TRIGGER v2_nodes_delete AFTER DELETE ON v2_nodes BEGIN UPDATE v2_state SET stored_bytes=stored_bytes-OLD.plaintext_size,node_count=node_count -1 WHERE execution_id=OLD.execution_id; END;

CREATE TRIGGER v2_nodes_update AFTER UPDATE ON v2_nodes BEGIN UPDATE v2_state SET stored_bytes=stored_bytes+NEW.plaintext_size-OLD.plaintext_size WHERE execution_id=NEW.execution_id; END;

CREATE TRIGGER v2_inputs_insert AFTER INSERT ON v2_inputs BEGIN UPDATE v2_state SET stored_bytes=stored_bytes+NEW.plaintext_size WHERE execution_id=NEW.execution_id; END;

CREATE TRIGGER v2_inputs_delete AFTER DELETE ON v2_inputs BEGIN UPDATE v2_state SET stored_bytes=stored_bytes-OLD.plaintext_size WHERE execution_id=OLD.execution_id; END;

CREATE TRIGGER v2_inputs_update AFTER UPDATE ON v2_inputs BEGIN UPDATE v2_state SET stored_bytes=stored_bytes+NEW.plaintext_size-OLD.plaintext_size WHERE execution_id=NEW.execution_id; END;

CREATE INDEX executions_v2_pending ON executions(created_at,id) WHERE api_version=2 AND status='pending';
