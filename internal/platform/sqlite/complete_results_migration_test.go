package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/javleds/api-engine/migrations"
	"path/filepath"
	"testing"
)

func TestCompleteResultsMigrationPreservesLegacyDeliveries(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "legacy.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for index, name := range []string{"engine/000001_initial.up.sql", "engine/000002_v2.up.sql"} {
		raw, err := migrations.Files.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(raw)); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(raw)
		if _, err = db.Exec(`INSERT INTO schema_migrations(version,checksum,applied_at) VALUES(?,?,0)`, index+1, hex.EncodeToString(digest[:])); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`INSERT INTO executions(id,owner_id,status,definition_version,webhook_enabled,request_fingerprint,snapshot_blob,created_at,metadata_expires_at,api_version) VALUES('legacy','owner','running',1,1,'fingerprint',x'01',0,100,2)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO v2_state(execution_id,deadline_at) VALUES('legacy',100)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO webhook_deliveries(id,event_id,execution_id,owner_id,event_type,target_blob,payload_blob,status,attempts,next_attempt_at,created_at,deadline_at) VALUES('delivery','event','legacy','owner','execution.completed',x'0102',x'0304','retrying',3,5,1,100)`); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db, "engine"); err != nil {
		t.Fatal(err)
	}
	var target, payload []byte
	var status string
	var attempts int
	if err = db.QueryRow(`SELECT target_blob,payload_blob,status,attempts FROM webhook_deliveries WHERE id='delivery'`).Scan(&target, &payload, &status, &attempts); err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(target) != "0102" || hex.EncodeToString(payload) != "0304" || status != "retrying" || attempts != 3 {
		t.Fatal("legacy delivery changed")
	}
	var enabled bool
	if err = db.QueryRow(`SELECT results_enabled FROM v2_state WHERE execution_id='legacy'`).Scan(&enabled); err != nil || enabled {
		t.Fatalf("legacy gained new semantics: %v %v", enabled, err)
	}
	if _, err = db.Exec(`INSERT INTO webhook_deliveries(id,event_id,execution_id,owner_id,event_type,target_blob,payload_blob,status,next_attempt_at,created_at,deadline_at) VALUES('duplicate','duplicate','legacy','owner','execution.completed',x'01',x'02','pending',1,1,100)`); err == nil {
		t.Fatal("duplicate terminal accepted")
	}
	if err = Migrate(ctx, db, "engine"); err != nil {
		t.Fatal(err)
	}
}
