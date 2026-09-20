package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"

	d "github.com/javleds/api-engine/internal/execution/domain"
)

// Completed projections are immutable. Only one sibling page and traversal
// frames are retained, even when independently produced chunks interleave.
func (s *ExecutionStore) writeV2TreeResult(ctx context.Context, owner string, id d.ExecutionID, result d.V2Result, w io.Writer) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if _, err = w.Write(raw[:len(raw)-1]); err != nil {
		return err
	}
	if _, err = io.WriteString(w, `,"data":`); err != nil {
		return err
	}
	var blob []byte
	if err = s.db.QueryRowContext(ctx, `SELECT payload_blob FROM v2_nodes WHERE execution_id=? AND node_id=?`, id, result.NodeID).Scan(&blob); err != nil {
		return err
	}
	raw, err = s.open(ctx, "v2_nodes", owner, string(id)+":"+result.NodeID, blob)
	if err != nil {
		return err
	}
	var root d.V2Node
	if err = json.Unmarshal(raw, &root); err != nil {
		return err
	}
	stream := resultJSONStream{writer: w}
	if err = writeProjectedResultNode(&stream, root); err != nil {
		return err
	}
	var stack []resultTreeFrame
	if root.Kind != "value" {
		stack = append(stack, resultTreeFrame{node: root.NodeID})
	}
	for len(stack) > 0 {
		frame := &stack[len(stack)-1]
		nodes, positions, err := s.resultTreeChildren(ctx, owner, id, *frame)
		if err != nil {
			return err
		}
		if len(nodes) == 0 {
			if err = stream.write(d.V2Chunk{Operation: "close", NodeID: frame.node}); err != nil {
				return err
			}
			stack = stack[:len(stack)-1]
			continue
		}
		for i, node := range nodes {
			if err = writeProjectedResultNode(&stream, node); err != nil {
				return err
			}
			frame.position, frame.afterNode = positions[i], node.NodeID
			if node.Kind != "value" {
				stack = append(stack, resultTreeFrame{node: node.NodeID})
				// Discard the remaining siblings before descending. Requery from the
				// last consumed position on ascent instead of retaining ancestor pages.
				break
			}
		}
	}
	if _, err = io.WriteString(w, `,"errors":[`); err != nil {
		return err
	}
	if result.ErrorSummary.Total > 0 {
		if err = s.writeResultTreeErrors(ctx, owner, id, result.NodeID, w); err != nil {
			return err
		}
	}
	_, err = io.WriteString(w, `],"complete":true}`)
	return err
}

func writeProjectedResultNode(stream *resultJSONStream, node d.V2Node) error {
	if node.Status == "pending" {
		return errors.New("incomplete result projection")
	}
	if node.Kind == "value" && !json.Valid(node.Value) {
		return errors.New("invalid result value")
	}
	return stream.write(d.V2Chunk{Operation: node.Kind, NodeID: node.NodeID, ParentID: node.ParentID, Key: node.Key, Index: node.Index, Value: node.Value})
}

func (s *ExecutionStore) resultTreeChildren(ctx context.Context, owner string, id d.ExecutionID, frame resultTreeFrame) ([]d.V2Node, []int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT node_id,position,payload_blob FROM v2_nodes WHERE execution_id=? AND parent_id=? AND (position>? OR (position=? AND node_id>?)) ORDER BY position,node_id LIMIT 100`, id, frame.node, frame.position, frame.position, frame.afterNode)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var nodes []d.V2Node
	var positions []int64
	size := 0
	for rows.Next() {
		var key string
		var position int64
		var blob []byte
		if err = rows.Scan(&key, &position, &blob); err != nil {
			return nil, nil, err
		}
		raw, err := s.open(ctx, "v2_nodes", owner, string(id)+":"+key, blob)
		if err != nil {
			return nil, nil, err
		}
		if len(nodes) > 0 && size+len(raw) > 1<<20 {
			break
		}
		var node d.V2Node
		if err = json.Unmarshal(raw, &node); err != nil {
			return nil, nil, err
		}
		nodes = append(nodes, node)
		positions = append(positions, position)
		size += len(raw)
	}
	return nodes, positions, rows.Err()
}

func (s *ExecutionStore) writeResultTreeErrors(ctx context.Context, owner string, id d.ExecutionID, node string, w io.Writer) error {
	var after int64
	first := true
	for {
		page, next, err := s.resultTreeErrors(ctx, owner, id, node, after)
		if err != nil {
			return err
		}
		if len(page) == 0 {
			return nil
		}
		for _, raw := range page {
			if !first {
				if _, err = io.WriteString(w, ","); err != nil {
					return err
				}
			}
			if _, err = w.Write(raw); err != nil {
				return err
			}
			first = false
		}
		after = next
	}
}

func (s *ExecutionStore) resultTreeErrors(ctx context.Context, owner string, id d.ExecutionID, node string, after int64) ([]json.RawMessage, int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT sequence,payload_blob FROM v2_result_errors WHERE execution_id=? AND result_node_id=? AND sequence>? ORDER BY sequence LIMIT 100`, id, node, after)
	if err != nil {
		return nil, after, err
	}
	defer rows.Close()
	var page []json.RawMessage
	size := 0
	for rows.Next() {
		var sequence int64
		var blob []byte
		if err = rows.Scan(&sequence, &blob); err != nil {
			return nil, after, err
		}
		raw, err := s.open(ctx, "v2_result_errors", owner, string(id)+":"+node+":"+strconv.FormatInt(sequence, 10), blob)
		if err != nil {
			return nil, after, err
		}
		if len(page) > 0 && size+len(raw) > 1<<20 {
			break
		}
		if !json.Valid(raw) {
			return nil, after, errors.New("invalid result diagnostic")
		}
		page = append(page, raw)
		after = sequence
		size += len(raw)
	}
	return page, after, rows.Err()
}
