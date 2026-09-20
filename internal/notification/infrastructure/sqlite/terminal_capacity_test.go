package sqlite

import (
	"context"
	"testing"
)

func TestGatedTerminalDoesNotDeadlockSingleNodeAdmissionSlot(t *testing.T) {
	for _, ordering := range []string{"completion", "source"} {
		t.Run(ordering, func(t *testing.T) {
			o, now := orderedOutbox(t, ordering, 1)
			node := addOrderedResult(t, o, now, 1, 0, 0)
			if _, err := o.db.Exec(`UPDATE v2_state SET terminal=1 WHERE execution_id='execution'`); err != nil {
				t.Fatal(err)
			}
			claimOrdered(t, o, now, "")
			if !projectOrderedResult(t, o, node, ordering == "source", 1) {
				t.Fatal("gated terminal consumed the only node-admission slot")
			}
			delivery := claimOrdered(t, o, now, "node-1")
			if err := o.SaveAttempt(context.Background(), *delivery, true, nil, 202, "", now); err != nil {
				t.Fatal(err)
			}
			claimOrdered(t, o, now, "delivery")
		})
	}
}

func TestGatedTerminalBacklogDoesNotExemptUnresolvedNodesFromCapacity(t *testing.T) {
	o, now := orderedOutbox(t, "completion", 1)
	first := addOrderedResult(t, o, now, 1, 0, 0)
	second := addOrderedResult(t, o, now, 2, 0, 1)
	third := addOrderedResult(t, o, now, 3, 0, 2)
	if _, err := o.db.Exec(`UPDATE v2_state SET terminal=1 WHERE execution_id='execution'`); err != nil {
		t.Fatal(err)
	}
	// Two accepted jobs have finished before the subscriber projects their nodes.
	// Their terminal backlog alone fills the configured limit of two deliveries.
	if _, err := o.db.Exec(`INSERT INTO executions(id,owner_id,status,definition_version,webhook_enabled,request_fingerprint,snapshot_blob,created_at,metadata_expires_at,api_version,webhook_ordering,webhook_max_in_flight)
 SELECT 'second',owner_id,status,definition_version,webhook_enabled,'second',snapshot_blob,created_at,metadata_expires_at,api_version,webhook_ordering,webhook_max_in_flight FROM executions WHERE id='execution'`); err != nil {
		t.Fatal(err)
	}
	if _, err := o.db.Exec(`INSERT INTO v2_state(execution_id,deadline_at,terminal,results_enabled,last_result_sequence) SELECT 'second',deadline_at,1,1,1 FROM v2_state WHERE execution_id='execution'`); err != nil {
		t.Fatal(err)
	}
	if _, err := o.db.Exec(`INSERT INTO v2_results(execution_id,node_id,sequence,event_id,payload_blob,plaintext_size,first_chunk,last_chunk,occurred_at) SELECT 'second','other-node',1,'other-event',payload_blob,plaintext_size,first_chunk,last_chunk,occurred_at FROM v2_results WHERE execution_id='execution' AND sequence=1`); err != nil {
		t.Fatal(err)
	}
	if _, err := o.db.Exec(`INSERT INTO webhook_deliveries(id,event_id,execution_id,owner_id,event_type,target_blob,payload_blob,status,next_attempt_at,created_at,deadline_at) SELECT 'other-terminal','other-terminal','second',owner_id,event_type,target_blob,payload_blob,status,next_attempt_at,created_at,deadline_at FROM webhook_deliveries WHERE id='delivery'`); err != nil {
		t.Fatal(err)
	}
	if !projectOrderedResult(t, o, first, false, 2) || !projectOrderedResult(t, o, second, false, 2) {
		t.Fatal("terminal backlog prevented bounded node admission")
	}
	if projectOrderedResult(t, o, third, false, 2) {
		t.Fatal("terminal exemption incorrectly exempted pending nodes")
	}
	var occupancy, rows int
	if err := o.db.QueryRowContext(context.Background(), nodeAdmissionOccupancySQL).Scan(&occupancy); err != nil || occupancy != 2 {
		t.Fatal("node capacity accounting", occupancy, err)
	}
	if err := o.db.QueryRow(`SELECT count(*) FROM webhook_deliveries WHERE status='pending'`).Scan(&rows); err != nil || rows != 4 {
		t.Fatal("deferred terminals must remain durable", rows, err)
	}
}
