package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	"io"
	"strconv"
)

func (m *v2Machine) stepCall(ctx context.Context, frame v2Frame, index int, commit *execution.V2Commit) (bool, error) {
	call := m.call(frame.Call)
	switch frame.Phase {
	case "":
		if len(frame.Call) == 1 && call.Type == "single" {
			frame.FirstChunk = m.sequence + int64(len(commit.Chunks)) + 1
		}
		if call.Pagination != nil {
			frame.Page = call.Pagination.Start
			if call.Pagination.PageSize != "" {
				text, _ := definition.ScalarString(call.Request.Query[call.Pagination.PageSize])
				_, _ = fmt.Sscan(text, &frame.Expected)
			}
		}
		frame.Phase = "reserve"
		m.checkpoint.Stack[index] = frame
	case "reserve":
		if m.parallel {
			commit.LogicalReservations++
			commit.MaxLogicalRequests = int64(m.engine.Options.MaxLogicalRequests)
		} else if err := m.runner.V2Store.ReserveV2Budget(ctx, m.id, m.generation, "logical", int64(m.engine.Options.MaxLogicalRequests)); err != nil {
			return false, err
		}
		frame.Phase = "fetch"
		m.checkpoint.Stack[index] = frame
		return true, nil
	case "fetch":
		return m.fetchPage(ctx, frame, index, commit)
	case "items":
		value, err := m.input(ctx, frame.Ref)
		if err != nil {
			return false, err
		}
		if call.Type == "collection" {
			items := value.([]any)
			if frame.Cursor < len(items) {
				position := frame.Offset + int64(frame.Cursor)
				ref := v2ChildRef(frame.Ref, strconv.Itoa(frame.Cursor))
				path := v2Path(frame.Path, strconv.FormatInt(position, 10))
				childFrame := v2Frame{Kind: "item", Call: frame.Call, Ref: ref, Ancestors: frame.Ancestors, NodeID: m.nodeID(path), ParentID: frame.NodeID, Index: &position, Path: path, ErrorStart: m.checkpoint.Report.ErrorSummary.Total}
				if m.fork != nil {
					if !m.fork(childFrame, commit) {
						return true, nil
					}
				} else {
					m.push(childFrame)
				}
				frame.Cursor++
				m.checkpoint.Stack[index] = frame
				return false, nil
			}
			if m.waitChildren != nil && m.waitChildren() {
				return true, nil
			}
			if !frame.Stop {
				frame.Offset += int64(len(items))
				frame.Page++
				frame.Cursor = 0
				frame.Phase = "reserve"
				m.checkpoint.Stack[index] = frame
				return false, nil
			}
		} else if frame.Cursor == 0 {
			frame.Cursor++
			m.checkpoint.Stack[index] = frame
			item := frame
			item.Kind = "item"
			item.Phase = ""
			item.Cursor = 0
			m.push(item)
			return false, nil
		}
		frame.Phase = "close"
		m.checkpoint.Stack[index] = frame
	case "close":
		if call.Type == "collection" {
			m.chunk(frame, "close", nil, commit)
			if !frame.Available {
				commit.Chunks[len(commit.Chunks)-1].Status = "failed"
			}
		}
		if len(frame.Call) == 1 {
			root := &m.checkpoint.Report.Roots[frame.Call[0]]
			root.DataAvailable = frame.Available
			switch {
			case !frame.Available:
				root.Status = execution.Failed
				m.checkpoint.Progress.FailedRootCalls++
			case m.checkpoint.Report.ErrorSummary.Total > frame.ErrorStart:
				root.Status = execution.CompletedWithErrors
				m.checkpoint.Progress.PartialRootCalls++
			default:
				root.Status = execution.Completed
				m.checkpoint.Progress.SuccessfulRootCalls++
			}
			if call.Type == "single" {
				status := ""
				if !frame.Available {
					status = "failed"
				}
				// Assembly failures publish a failed null even when upstream data arrived.
				if frame.Available && m.checkpoint.Report.ErrorSummary.Total > frame.ErrorStart && m.checkpoint.SingleFailed {
					status = "failed"
				}
				m.finishResult(frame, status, commit)
				m.checkpoint.SingleFailed = false
			}
			m.checkpoint.Root++
		}
		m.pop()
	default:
		return false, errors.New("unsupported v2 call phase")
	}
	return false, nil
}

func (m *v2Machine) fetchPage(ctx context.Context, frame v2Frame, index int, commit *execution.V2Commit) (bool, error) {
	call := m.call(frame.Call)
	values := make(map[string]any, len(frame.Ancestors))
	for name, ref := range frame.Ancestors {
		value, err := m.input(ctx, ref)
		if err != nil {
			return false, err
		}
		values[name] = value
	}
	n := NodeContext{Path: frame.Path, Values: values}
	if call.Pagination != nil {
		n.Page = &frame.Page
	}
	processor := CallProcessor{engine: m.engine, definition: m.definition, ownerID: m.owner, executionID: string(m.id)}
	request, err := processor.buildRequest(ctx, call, n)
	if err != nil {
		m.failCall(frame, index, mapFetchFailure(call, n, err), commit)
		return false, nil
	}
	request.BeforeAttempt = func(attemptCtx context.Context) error {
		return m.runner.V2Store.ReserveV2Budget(attemptCtx, m.id, m.generation, "attempt", int64(m.engine.Options.MaxHTTPAttempts))
	}
	request.OnAttempt = nil
	request.OnRetry = nil
	response, err := m.engine.Fetcher.Fetch(ctx, request)
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		if errors.Is(err, execution.ErrV2Limit) {
			return false, err
		}
		var upstream *fetch.UpstreamFailure
		if !errors.As(err, &upstream) && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) && err.Error() != "FETCH_CAPACITY_EXCEEDED" {
			return false, err
		}
		if upstream != nil && upstream.Code == "INTERNAL_WORKER_PANIC" {
			return false, errors.New("INTERNAL_WORKER_PANIC")
		}
		m.failCall(frame, index, mapFetchFailure(call, n, err), commit)
		return false, nil
	}
	if response.CacheHit {
		m.checkpoint.Progress.CacheHits++
	}
	if response.ReusedInFlight {
		m.checkpoint.Progress.CoalescedRequests++
	}
	if response.Response.Attempts > 1 {
		m.checkpoint.Progress.Retries += uint64(response.Response.Attempts - 1)
	}
	decoder := json.NewDecoder(bytes.NewReader(response.Response.Body))
	decoder.UseNumber()
	var value any
	if err = decoder.Decode(&value); err == nil {
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			err = errors.New("trailing JSON")
		}
	}
	if err != nil {
		m.failCall(frame, index, nodeFailure(call, n, "UPSTREAM_INVALID_JSON", "fetch", response.Response.Attempts), commit)
		return false, nil
	}
	selected, err := definition.Select(value, call.Select)
	if err != nil {
		m.failCall(frame, index, nodeFailure(call, n, "SELECT_PATH_NOT_FOUND", "select", response.Response.Attempts), commit)
		return false, nil
	}
	items, array := selected.([]any)
	if (call.Type == "collection") != array {
		m.failCall(frame, index, nodeFailure(call, n, "TYPE_MISMATCH", "type", response.Response.Attempts), commit)
		return false, nil
	}
	raw, err := json.Marshal(selected)
	if err != nil {
		return false, err
	}
	frame.Stop = call.Pagination == nil
	if call.Pagination != nil {
		size := int64(len(items))
		if frame.Expected == 0 && size > 0 {
			frame.Expected = size
		}
		if frame.Expected > 0 && size > frame.Expected {
			m.failCall(frame, index, nodeFailure(call, n, "PAGINATION_SIZE_INCONSISTENT", "pagination", response.Response.Attempts), commit)
			return false, nil
		}
		digest := sha256.Sum256(raw)
		hash := hex.EncodeToString(digest[:])
		if m.engine.Options.DetectRepeatedPages && frame.Accepted > 0 && size == frame.Expected && size > 0 && hash == frame.Previous {
			m.failCall(frame, index, nodeFailure(call, n, "PAGINATION_NO_PROGRESS", "pagination", response.Response.Attempts), commit)
			return false, nil
		}
		frame.Previous = hash
		frame.Accepted++
		frame.Stop = size == 0 || size < frame.Expected || !call.Pagination.Pages.All && frame.Accepted >= call.Pagination.Pages.Count
		if !frame.Stop && frame.Accepted >= int64(m.engine.Options.MaxPagesPerCall) {
			frame.Stop = true
			m.failure(frame, "PAGINATION_LIMIT_REACHED", "pagination", commit)
		}
	}
	if !frame.Available && call.Type == "collection" {
		m.chunk(frame, "array", nil, commit)
	}
	frame.Available = true
	if len(frame.Call) == 1 {
		m.checkpoint.Report.Roots[frame.Call[0]].DataAvailable = true
	}
	key := frame.NodeID + ":" + strconv.FormatInt(frame.Page, 10)
	commit.Inputs = append(commit.Inputs, execution.V2Input{Key: key, Payload: raw})
	frame.Ref = v2InputRef{Key: key}
	frame.Cursor = 0
	frame.Phase = "items"
	m.checkpoint.Stack[index] = frame
	m.cachedKey = key
	m.cachedValue = selected
	return true, nil
}
func (m *v2Machine) failCall(frame v2Frame, index int, failure execution.NodeFailure, commit *execution.V2Commit) {
	m.recordFailure(frame, failure, commit)
	if !frame.Available {
		if m.call(frame.Call).Type == "collection" {
			m.chunk(frame, "array", nil, commit)
		} else {
			m.chunk(frame, "value", nil, commit)
			commit.Chunks[len(commit.Chunks)-1].Status = "failed"
		}
	}
	frame.Phase = "close"
	m.checkpoint.Stack[index] = frame
}
