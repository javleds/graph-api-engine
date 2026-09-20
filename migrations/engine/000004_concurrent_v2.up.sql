ALTER TABLE executions ADD COLUMN webhook_ordering TEXT NOT NULL DEFAULT '';
ALTER TABLE executions ADD COLUMN webhook_max_in_flight INTEGER NOT NULL DEFAULT 0;
ALTER TABLE v2_state ADD COLUMN source_root INTEGER NOT NULL DEFAULT 0;
ALTER TABLE v2_state ADD COLUMN source_index INTEGER NOT NULL DEFAULT -1;
ALTER TABLE v2_results ADD COLUMN storage_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE v2_results ADD COLUMN source_root INTEGER NOT NULL DEFAULT 0;
ALTER TABLE v2_results ADD COLUMN source_index INTEGER NOT NULL DEFAULT -1;
CREATE INDEX v2_results_source ON v2_results(execution_id,source_root,source_index);
CREATE INDEX webhook_deliveries_execution_status ON webhook_deliveries(execution_id,status);
CREATE TABLE v2_result_errors (
 execution_id TEXT NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
 result_node_id TEXT NOT NULL,
 sequence INTEGER NOT NULL,
 payload_blob BLOB NOT NULL,
 plaintext_size INTEGER NOT NULL,
 PRIMARY KEY(execution_id,result_node_id,sequence)
);
CREATE TRIGGER v2_result_errors_insert AFTER INSERT ON v2_result_errors BEGIN UPDATE v2_state SET stored_bytes=stored_bytes+NEW.plaintext_size WHERE execution_id=NEW.execution_id; END;
CREATE TRIGGER v2_result_errors_delete AFTER DELETE ON v2_result_errors BEGIN UPDATE v2_state SET stored_bytes=stored_bytes-OLD.plaintext_size WHERE execution_id=OLD.execution_id; END;
