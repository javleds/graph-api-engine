package worker

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	"sort"
	"strconv"
	"strings"
)

type v2Machine struct {
	runner      *V2Runner
	engine      *Engine
	definition  definition.CompiledDefinition
	id          execution.ExecutionID
	owner       string
	generation  int64
	checkpoint  *v2Checkpoint
	cachedKey   string
	cachedValue any
}

func (m *v2Machine) nodeID(path string) string {
	mac := hmac.New(sha256.New, m.engine.IdentityKey)
	mac.Write([]byte(string(m.id) + "\x00" + path))
	return hex.EncodeToString(mac.Sum(nil)[:16])
}
func v2Path(parent, key string) string {
	return parent + "/" + strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}
func (m *v2Machine) call(indices []int) definition.CompiledCall {
	call := m.definition.Roots[indices[0]]
	for _, i := range indices[1:] {
		call = call.Children[i]
	}
	return call
}
func (m *v2Machine) push(frame v2Frame) { m.checkpoint.Stack = append(m.checkpoint.Stack, frame) }
func (m *v2Machine) pop()               { m.checkpoint.Stack = m.checkpoint.Stack[:len(m.checkpoint.Stack)-1] }
func (m *v2Machine) chunk(frame v2Frame, operation string, value any, commit *execution.V2Commit) {
	chunk := execution.V2Chunk{Type: "data", Operation: operation, NodeID: frame.NodeID, ParentID: frame.ParentID, Key: frame.Key, Index: frame.Index, Path: frame.Path}
	if operation == "value" {
		chunk.Value, _ = json.Marshal(value)
	}
	if operation == "close" {
		chunk.Status = "completed"
		if m.checkpoint.Report.ErrorSummary.Total > frame.ErrorStart {
			chunk.Status = "completed_with_errors"
		}
	}
	commit.Chunks = append(commit.Chunks, chunk)
}
func (m *v2Machine) failure(frame v2Frame, code, stage string, commit *execution.V2Commit) {
	call := m.call(frame.Call)
	failure := nodeFailure(call, NodeContext{Path: frame.Path}, code, stage, 0)
	m.recordFailure(frame, failure, commit)
}
func (m *v2Machine) recordFailure(frame v2Frame, failure execution.NodeFailure, commit *execution.V2Commit) {
	report := &m.checkpoint.Report
	report.ErrorSummary.Total++
	if len(report.Errors) < m.engine.Options.MaxDetails {
		candidate := append(append([]execution.NodeFailure(nil), report.Errors...), failure)
		raw, _ := json.Marshal(candidate)
		if len(raw) < m.engine.Options.MaxReportBytes/2 {
			report.Errors = candidate
		}
	}
	report.ErrorSummary.Returned = uint64(len(report.Errors))
	report.ErrorSummary.Truncated = report.ErrorSummary.Returned < report.ErrorSummary.Total
	raw, _ := json.Marshal(failure)
	commit.Chunks = append(commit.Chunks, execution.V2Chunk{Type: "error", NodeID: frame.NodeID, Path: frame.Path, Value: raw})
}
func (m *v2Machine) input(ctx context.Context, ref v2InputRef) (any, error) {
	if ref.Key != m.cachedKey {
		raw, err := m.runner.V2Store.ReadV2Input(ctx, m.id, ref.Key)
		if err != nil {
			return nil, err
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		var value any
		if err = decoder.Decode(&value); err != nil {
			return nil, err
		}
		m.cachedKey = ref.Key
		m.cachedValue = value
	}
	value := m.cachedValue
	for _, part := range ref.Pointer {
		switch typed := value.(type) {
		case map[string]any:
			value = typed[part]
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(typed) {
				return nil, errors.New("invalid checkpoint input pointer")
			}
			value = typed[i]
		default:
			return nil, errors.New("invalid checkpoint input pointer")
		}
	}
	if ref.Normalize {
		switch value.(type) {
		case string, json.Number, bool:
			value = map[string]any{"value": value}
		}
	}
	return value, nil
}
func v2ChildRef(parent v2InputRef, key string) v2InputRef {
	parent.Pointer = append(append([]string(nil), parent.Pointer...), key)
	parent.Normalize = false
	return parent
}

func (m *v2Machine) step(ctx context.Context, commit *execution.V2Commit) (boundary bool, err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("INTERNAL_WORKER_PANIC")
		}
	}()
	if len(m.checkpoint.Stack) == 0 {
		if m.checkpoint.Root >= len(m.definition.Roots) {
			return true, nil
		}
		root := m.definition.Roots[m.checkpoint.Root]
		path := v2Path("", root.Name)
		m.checkpoint.Report.Roots[m.checkpoint.Root].Status = execution.Running
		m.push(v2Frame{Kind: "call", Call: []int{m.checkpoint.Root}, NodeID: m.nodeID(path), Key: root.Name, Path: path, ErrorStart: m.checkpoint.Report.ErrorSummary.Total})
	}
	index := len(m.checkpoint.Stack) - 1
	frame := m.checkpoint.Stack[index]
	switch frame.Kind {
	case "call":
		return m.stepCall(ctx, frame, index, commit)
	case "item":
		return false, m.stepItem(ctx, frame, index, commit)
	case "value":
		return false, m.stepValue(ctx, frame, index, commit)
	default:
		return false, fmt.Errorf("unsupported v2 continuation %q", frame.Kind)
	}
}

func (m *v2Machine) stepValue(ctx context.Context, frame v2Frame, index int, commit *execution.V2Commit) error {
	value, err := m.input(ctx, frame.Ref)
	if err != nil {
		return err
	}
	// Scalar normalization is virtual, so its value member references the original.
	switch typed := value.(type) {
	case map[string]any:
		if frame.Phase == "" {
			m.chunk(frame, "object", nil, commit)
			frame.Phase = "members"
			m.checkpoint.Stack[index] = frame
			return nil
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if frame.Cursor < len(keys) {
			key := keys[frame.Cursor]
			frame.Cursor++
			m.checkpoint.Stack[index] = frame
			ref := v2ChildRef(frame.Ref, key)
			if frame.Ref.Normalize {
				original := frame.Ref
				original.Normalize = false
				rawValue, err := m.input(ctx, original)
				if err != nil {
					return err
				}
				if _, ok := rawValue.(map[string]any); !ok {
					ref = original
				}
			}
			path := v2Path(frame.Path, key)
			m.push(v2Frame{Kind: "value", Ref: ref, NodeID: m.nodeID(path), ParentID: frame.NodeID, Key: key, Path: path, ErrorStart: frame.ErrorStart})
			return nil
		}
	case []any:
		if frame.Phase == "" {
			m.chunk(frame, "array", nil, commit)
			frame.Phase = "members"
			m.checkpoint.Stack[index] = frame
			return nil
		}
		if frame.Cursor < len(typed) {
			position := int64(frame.Cursor)
			frame.Cursor++
			m.checkpoint.Stack[index] = frame
			path := v2Path(frame.Path, strconv.FormatInt(position, 10))
			m.push(v2Frame{Kind: "value", Ref: v2ChildRef(frame.Ref, strconv.FormatInt(position, 10)), NodeID: m.nodeID(path), ParentID: frame.NodeID, Index: &position, Path: path, ErrorStart: frame.ErrorStart})
			return nil
		}
	default:
		m.chunk(frame, "value", value, commit)
		m.pop()
		return nil
	}
	if !frame.Stop {
		m.chunk(frame, "close", nil, commit)
	}
	m.pop()
	return nil
}

func (m *v2Machine) stepItem(ctx context.Context, frame v2Frame, index int, commit *execution.V2Commit) error {
	call := m.call(frame.Call)
	if frame.Phase == "" {
		value, err := m.input(ctx, frame.Ref)
		if err != nil {
			return err
		}
		if len(call.Children) > 0 {
			var object map[string]any
			switch typed := value.(type) {
			case map[string]any:
				object = typed
			case string, json.Number, bool:
				object = map[string]any{"value": typed}
				frame.Ref.Normalize = true
			default:
				m.failure(frame, "PARENT_NOT_ENRICHABLE", "assembly", commit)
				m.chunk(frame, "value", nil, commit)
				m.pop()
				return nil
			}
			for _, child := range call.Children {
				if _, exists := object[child.Name]; exists {
					m.failure(frame, "OUTPUT_FIELD_COLLISION", "assembly", commit)
					m.chunk(frame, "value", nil, commit)
					m.pop()
					return nil
				}
			}
		}
		frame.Phase = "children"
		m.checkpoint.Stack[index] = frame
		valueFrame := frame
		valueFrame.Kind = "value"
		valueFrame.Phase = ""
		valueFrame.Stop = len(call.Children) > 0
		m.push(valueFrame)
		return nil
	}
	if frame.Cursor < len(call.Children) {
		childIndex := frame.Cursor
		child := call.Children[childIndex]
		frame.Cursor++
		m.checkpoint.Stack[index] = frame
		ancestors := make(map[string]v2InputRef, len(frame.Ancestors)+2)
		for key, ref := range frame.Ancestors {
			ancestors[key] = ref
		}
		ancestors[call.Name] = frame.Ref
		ancestors["parent"] = frame.Ref
		path := v2Path(frame.Path, child.Name)
		indices := append(append([]int(nil), frame.Call...), childIndex)
		m.push(v2Frame{Kind: "call", Call: indices, Ancestors: ancestors, NodeID: m.nodeID(path), ParentID: frame.NodeID, Key: child.Name, Path: path, ErrorStart: m.checkpoint.Report.ErrorSummary.Total})
		return nil
	}
	if len(call.Children) > 0 {
		m.chunk(frame, "close", nil, commit)
	}
	if len(frame.Call) == 1 {
		m.checkpoint.Progress.CompletedRootItems++
	}
	m.pop()
	return nil
}
