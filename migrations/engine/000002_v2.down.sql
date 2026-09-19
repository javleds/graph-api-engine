DROP TABLE v2_inputs;
DROP TABLE v2_nodes;
DROP TABLE v2_chunks;
DROP TABLE v2_tasks;
DROP TABLE v2_state;
DROP INDEX executions_v2_pending;
ALTER TABLE executions DROP COLUMN api_version;
