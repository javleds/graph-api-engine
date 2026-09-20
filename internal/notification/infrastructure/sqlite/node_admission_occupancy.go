package sqlite

// A terminal waiting for its node deliveries must not consume the capacity those
// nodes need to enter the outbox. Total pending rows can therefore exceed this
// node-admission cap by deferred terminals, bounded by accepted executions.
// Execution acceptance retains its original reservations and backlog accounting.
const nodeAdmissionOccupancySQL = `SELECT
 (SELECT count(*) FROM executions WHERE webhook_enabled=1 AND status IN ('pending','running'))+
 (SELECT count(*) FROM webhook_deliveries d
 JOIN executions e ON e.id=d.execution_id
 LEFT JOIN v2_state s ON s.execution_id=d.execution_id
 WHERE d.status IN ('pending','retrying','delivering')
 AND NOT (e.api_version=2 AND e.webhook_ordering<>'' AND d.event_type<>'node.finished'
   AND (coalesce(s.webhook_cursor<s.last_result_sequence,0) OR EXISTS (
     SELECT 1 FROM webhook_deliveries node WHERE node.execution_id=d.execution_id
     AND node.event_type='node.finished' AND node.status IN ('pending','retrying','delivering')
   ))))`
