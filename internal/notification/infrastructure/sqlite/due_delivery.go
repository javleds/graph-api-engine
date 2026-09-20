package sqlite

// Retrying deliveries retain their window position while waiting for backoff.
// Legacy executions have no policy and retain their independent delivery behavior.
const dueDeliverySQL = `SELECT
 d.id,d.event_id,d.execution_id,d.owner_id,d.event_type,d.target_blob,d.payload_blob,
 d.status,d.attempts,d.version,d.next_attempt_at,d.deadline_at
 FROM webhook_deliveries d
 JOIN executions e ON e.id=d.execution_id
 LEFT JOIN v2_state s ON s.execution_id=d.execution_id
 LEFT JOIN v2_results r ON r.event_id=d.event_id
 WHERE d.status IN ('pending','retrying') AND d.next_attempt_at<=? AND d.deadline_at>?
 AND (e.webhook_ordering='' OR (
   (d.status='retrying' OR (
     SELECT count(*) FROM webhook_deliveries occupied
     WHERE occupied.execution_id=d.execution_id AND occupied.status IN ('retrying','delivering')
   ) < CASE WHEN ?>0 THEN min(e.webhook_max_in_flight,?) ELSE e.webhook_max_in_flight END)
   AND (d.event_type='node.finished' OR (
     s.terminal=1 AND NOT EXISTS (
       SELECT 1 FROM v2_results missing WHERE missing.execution_id=d.execution_id
       AND NOT EXISTS (SELECT 1 FROM webhook_deliveries projected WHERE projected.event_id=missing.event_id))
     AND NOT EXISTS (SELECT 1 FROM webhook_deliveries pending
       WHERE pending.execution_id=d.execution_id AND pending.event_type='node.finished'
       AND pending.status IN ('pending','retrying','delivering'))
   ))
   AND (e.webhook_ordering<>'source' OR d.event_type<>'node.finished' OR (
     (s.terminal=1 OR r.source_root<s.source_root OR (r.source_root=s.source_root AND r.source_index<=s.source_index))
     AND NOT EXISTS (
       SELECT 1 FROM v2_results earlier WHERE earlier.execution_id=d.execution_id
       AND (earlier.source_root<r.source_root OR (earlier.source_root=r.source_root AND earlier.source_index<r.source_index))
       AND NOT EXISTS (SELECT 1 FROM webhook_deliveries admitted
         WHERE admitted.event_id=earlier.event_id AND (admitted.attempts>0 OR admitted.status='failed'))
     )
   ))
 ))
 ORDER BY CASE WHEN d.execution_id>? THEN 0 ELSE 1 END,d.execution_id,
 CASE WHEN e.webhook_ordering='source' THEN coalesce(r.source_root,9223372036854775807) ELSE 0 END,
 CASE WHEN e.webhook_ordering='source' THEN coalesce(r.source_index,9223372036854775807) ELSE 0 END,
 CASE WHEN e.webhook_ordering<>'' THEN coalesce(r.sequence,9223372036854775807) ELSE 0 END,
 d.next_attempt_at,d.id LIMIT 1`
