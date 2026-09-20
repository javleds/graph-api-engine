package sqlite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	d "github.com/javleds/api-engine/internal/execution/domain"
)

func TestV2ProjectedResultStreamsInterleavedBranchesInOriginalArrayOrder(t *testing.T) {
	s, db := testStore(t)
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snapshot := seedV2(t, s, "projected-order")
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	sequence := int64(0)
	appendChunk := func(chunk d.V2Chunk) {
		t.Helper()
		sequence++
		if err := s.projectV2Node(ctx, tx, "owner", snapshot.ID, sequence, chunk); err != nil {
			t.Fatal(err)
		}
	}
	appendChunk(d.V2Chunk{Operation: "array", NodeID: "root"})
	appendChunk(d.V2Chunk{Operation: "object", NodeID: "unrelated"})
	for i := 204; i >= 0; i-- {
		index := int64(i)
		node := fmt.Sprintf("element-%03d", i)
		appendChunk(d.V2Chunk{Operation: "object", NodeID: node, ParentID: "root", Index: &index})
		appendChunk(d.V2Chunk{Operation: "value", NodeID: node + "-number", ParentID: node, Key: "number", Value: json.RawMessage(`9007199254740993`)})
		appendChunk(d.V2Chunk{Operation: "value", NodeID: node + "-null", ParentID: node, Key: "empty", Value: json.RawMessage(`null`)})
		appendChunk(d.V2Chunk{Operation: "value", NodeID: node + "-index", ParentID: node, Key: "index", Value: json.RawMessage(fmt.Sprint(i))})
		appendChunk(d.V2Chunk{Operation: "value", NodeID: node + "-other", ParentID: "unrelated", Key: node, Value: json.RawMessage(`"not in result"`)})
		appendChunk(d.V2Chunk{Operation: "close", NodeID: node, Status: "completed"})
	}
	appendChunk(d.V2Chunk{Operation: "close", NodeID: "root", Status: "completed"})
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	if err = s.writeV2TreeResult(ctx, "owner", snapshot.ID, d.V2Result{NodeID: "root", StorageVersion: 2}, &body); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]json.RawMessage
	if err = json.Unmarshal(body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	var data []json.RawMessage
	if err = json.Unmarshal(envelope["data"], &data); err != nil {
		t.Fatal(err)
	}
	if string(envelope["complete"]) != "true" || len(data) != 205 {
		t.Fatalf("incomplete result: %d", len(data))
	}
	for i, item := range data {
		if string(item) != fmt.Sprintf(`{"number":9007199254740993,"empty":null,"index":%d}`, i) {
			t.Fatalf("corrupted element: %s", item)
		}
	}
	if strings.Contains(body.String(), "not in result") {
		t.Fatal("unrelated branch leaked into result")
	}
}

func TestV2ProjectedResultRejectsIncompleteAndSparseProjections(t *testing.T) {
	for _, scenario := range []string{"pending", "sparse"} {
		t.Run(scenario, func(t *testing.T) {
			s, db := testStore(t)
			ctx := context.Background()
			snapshot := seedV2(t, s, "invalid-projection")
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			index := int64(1)
			chunks := []d.V2Chunk{{Operation: "array", NodeID: "root"}, {Operation: "value", NodeID: "child", ParentID: "root", Index: &index, Value: json.RawMessage(`1`)}}
			if scenario == "sparse" {
				chunks = append(chunks, d.V2Chunk{Operation: "close", NodeID: "root", Status: "completed"})
			}
			for i, chunk := range chunks {
				if err = s.projectV2Node(ctx, tx, "owner", snapshot.ID, int64(i+1), chunk); err != nil {
					t.Fatal(err)
				}
			}
			if err = tx.Commit(); err != nil {
				t.Fatal(err)
			}
			var body bytes.Buffer
			if err = s.writeV2TreeResult(ctx, "owner", snapshot.ID, d.V2Result{NodeID: "root", StorageVersion: 2}, &body); err == nil {
				t.Fatal("invalid projection accepted")
			}
			if strings.Contains(body.String(), `"complete":true`) {
				t.Fatal("invalid projection marked complete")
			}
		})
	}
}

func TestV2ProjectedResultStreamsPagesLargerThanOneMiB(t *testing.T) {
	s, db := testStore(t)
	ctx := context.Background()
	snapshot := seedV2(t, s, "projection-bytes")
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	chunks := []d.V2Chunk{{Operation: "object", NodeID: "root"}}
	for i := 0; i < 12; i++ {
		key := fmt.Sprintf("key-%02d", i)
		value, _ := json.Marshal(strings.Repeat("x", 128<<10))
		chunks = append(chunks, d.V2Chunk{Operation: "value", NodeID: key, ParentID: "root", Key: key, Value: value})
	}
	chunks = append(chunks, d.V2Chunk{Operation: "close", NodeID: "root", Status: "completed"})
	for i, chunk := range chunks {
		if err = s.projectV2Node(ctx, tx, "owner", snapshot.ID, int64(i+1), chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	if err = s.writeV2TreeResult(ctx, "owner", snapshot.ID, d.V2Result{NodeID: "root", StorageVersion: 2}, &body); err != nil {
		t.Fatal(err)
	}
	if body.Len() < 1536<<10 || !json.Valid(body.Bytes()) {
		t.Fatalf("invalid large result: %d", body.Len())
	}
}

func TestV2ProjectedResultDiagnosticsArePagedAndIsolatedByChain(t *testing.T) {
	s, db := testStore(t)
	ctx := context.Background()
	snapshot := seedV2(t, s, "projection-errors")
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err = s.projectV2Node(ctx, tx, "owner", snapshot.ID, 1, d.V2Chunk{Operation: "value", NodeID: "root", Value: json.RawMessage(`null`), Status: "failed"}); err != nil {
		t.Fatal(err)
	}
	for _, node := range []string{"root", "unrelated"} {
		for sequence := 1; sequence <= 205; sequence++ {
			raw := []byte(fmt.Sprintf(`{"code":"failed","position":%d,"chain":%q}`, sequence, node))
			blob, err := s.seal(ctx, "v2_result_errors", "owner", fmt.Sprintf("%s:%s:%d", snapshot.ID, node, sequence), raw)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO v2_result_errors(execution_id,result_node_id,sequence,payload_blob,plaintext_size) VALUES(?,?,?,?,?)`, snapshot.ID, node, sequence, blob, len(raw)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	if err = s.writeV2TreeResult(ctx, "owner", snapshot.ID, d.V2Result{NodeID: "root", StorageVersion: 2, ErrorSummary: d.ErrorSummary{Total: 205}}, &body); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]json.RawMessage
	if err = json.Unmarshal(body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	var diagnostics []json.RawMessage
	if err = json.Unmarshal(envelope["errors"], &diagnostics); err != nil {
		t.Fatal(err)
	}
	if string(envelope["complete"]) != "true" || len(diagnostics) != 205 {
		t.Fatalf("wrong diagnostics count: %d", len(diagnostics))
	}
	for i, diagnostic := range diagnostics {
		if string(diagnostic) != fmt.Sprintf(`{"code":"failed","position":%d,"chain":"root"}`, i+1) {
			t.Fatalf("incorrect diagnostic: %s", diagnostic)
		}
	}
}
