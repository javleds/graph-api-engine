package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	d "github.com/javleds/api-engine/internal/execution/domain"
	"io"
	"strconv"
	"time"
)

// WriteV2Result streams confirmed content, then diagnostics if present.
// Memory is bounded by a chunk page plus nesting depth, not by subtree size.
func (s *ExecutionStore) WriteV2Result(ctx context.Context, owner string, id d.ExecutionID, node string, now time.Time, w io.Writer) error {
	result, err := s.ReadV2Result(ctx, owner, id, node, now)
	if err != nil {
		return err
	}
	if result.StorageVersion == 2 {
		return s.writeV2TreeResult(ctx, owner, id, result, w)
	}
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
	stream := resultJSONStream{writer: w}
	if err = s.visitResultChunks(ctx, owner, id, result, now, func(chunk d.V2Chunk) error {
		if chunk.Type != "data" {
			return nil
		}
		return stream.write(chunk)
	}); err != nil {
		return err
	}
	if !stream.started || len(stream.stack) != 0 {
		return errors.New("incomplete result chunk range")
	}
	if _, err = io.WriteString(w, `,"errors":[`); err != nil {
		return err
	}
	if result.ErrorSummary.Total > 0 {
		first := true
		if err = s.visitResultChunks(ctx, owner, id, result, now, func(chunk d.V2Chunk) error {
			if chunk.Type != "error" {
				return nil
			}
			if !first {
				if _, err := io.WriteString(w, ","); err != nil {
					return err
				}
			}
			first = false
			_, err := w.Write(chunk.Value)
			return err
		}); err != nil {
			return err
		}
	}
	_, err = io.WriteString(w, `],"complete":true}`)
	return err
}

func (s *ExecutionStore) visitResultChunks(ctx context.Context, owner string, id d.ExecutionID, result d.V2Result, now time.Time, visit func(d.V2Chunk) error) error {
	after := result.FirstChunk - 1
	for after < result.LastChunk {
		page, err := s.ReadV2Chunks(ctx, owner, id, after, 100, 1<<20, now)
		if err != nil {
			return err
		}
		if len(page.Chunks) == 0 {
			return errors.New("missing confirmed result chunks")
		}
		for _, raw := range page.Chunks {
			var chunk d.V2Chunk
			if err = json.Unmarshal(raw, &chunk); err != nil {
				return err
			}
			sequence, err := strconv.ParseInt(chunk.Sequence, 10, 64)
			if err != nil || sequence != after+1 {
				return errors.New("noncontiguous result chunks")
			}
			if sequence > result.LastChunk {
				break
			}
			if err = visit(chunk); err != nil {
				return err
			}
			after = sequence
		}
	}
	return nil
}

type resultJSONStream struct {
	writer  io.Writer
	stack   []resultContainer
	started bool
}

func (s *resultJSONStream) write(c d.V2Chunk) error {
	if c.Operation == "close" {
		if len(s.stack) == 0 || s.stack[len(s.stack)-1].node != c.NodeID {
			return errors.New("invalid result closure")
		}
		top := s.stack[len(s.stack)-1]
		s.stack = s.stack[:len(s.stack)-1]
		closing := "}"
		if top.kind == "array" {
			closing = "]"
		}
		_, err := io.WriteString(s.writer, closing)
		return err
	}
	if len(s.stack) > 0 {
		parent := &s.stack[len(s.stack)-1]
		if c.ParentID != parent.node {
			return errors.New("invalid result parent")
		}
		if parent.children > 0 {
			if _, err := io.WriteString(s.writer, ","); err != nil {
				return err
			}
		}
		if parent.kind == "object" {
			key, _ := json.Marshal(c.Key)
			if _, err := s.writer.Write(key); err != nil {
				return err
			}
			if _, err := io.WriteString(s.writer, ":"); err != nil {
				return err
			}
		} else if c.Index == nil || *c.Index != parent.children {
			return errors.New("invalid result array position")
		}
		parent.children++
	} else if s.started {
		return errors.New("multiple roots in result range")
	}
	s.started = true
	switch c.Operation {
	case "object", "array":
		s.stack = append(s.stack, resultContainer{node: c.NodeID, kind: c.Operation})
		opening := "{"
		if c.Operation == "array" {
			opening = "["
		}
		_, err := io.WriteString(s.writer, opening)
		return err
	case "value":
		_, err := s.writer.Write(c.Value)
		return err
	default:
		return errors.New("unsupported result operation")
	}
}
