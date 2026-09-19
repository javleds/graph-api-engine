package worker

import (
	"context"
	"encoding/json"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func v2TestRunner(fake *fakeFetcher, roots ...definition.CompiledCall) (*V2Runner, *memoryExecutionStore, *v2MemoryStore) {
	raw, _ := json.Marshal(compiled(roots...))
	store := &memoryExecutionStore{snapshot: execution.ExecutionSnapshot{ID: "id", OwnerID: "owner", Definition: raw, APIVersion: 2}, current: execution.Execution{ID: "id", Status: execution.Pending, Version: 1, APIVersion: 2}}
	durable := newV2MemoryStore()
	runner := &V2Runner{Store: store, V2Store: durable, Finalizer: store, Engine: &Engine{Fetcher: fake}}
	return runner, store, durable
}
func v2Replay(chunks []execution.V2Chunk) map[string]any {
	nodes := map[string]execution.V2Chunk{}
	children := map[string][]string{}
	for _, chunk := range chunks {
		if chunk.Type != "data" || chunk.Operation == "close" {
			continue
		}
		if _, exists := nodes[chunk.NodeID]; !exists {
			children[chunk.ParentID] = append(children[chunk.ParentID], chunk.NodeID)
		}
		nodes[chunk.NodeID] = chunk
	}
	var build func(string) any
	build = func(id string) any {
		node := nodes[id]
		switch node.Operation {
		case "object":
			value := map[string]any{}
			for _, child := range children[id] {
				value[nodes[child].Key] = build(child)
			}
			return value
		case "array":
			ids := children[id]
			sort.Slice(ids, func(i, j int) bool { return *nodes[ids[i]].Index < *nodes[ids[j]].Index })
			value := make([]any, len(ids))
			for i, child := range ids {
				value[i] = build(child)
			}
			return value
		default:
			var value any
			decoder := json.NewDecoder(strings.NewReader(string(node.Value)))
			decoder.UseNumber()
			_ = decoder.Decode(&value)
			return value
		}
	}
	result := map[string]any{}
	for _, root := range children[""] {
		result[nodes[root].Key] = build(root)
	}
	return result
}
func TestV2ReplayMatchesV1AcrossNormalizationAndFailures(t *testing.T) {
	fixture := func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		if strings.HasSuffix(r.Request.URL, "/root") {
			return response(`[{"id":1,"nested":[{"x":2,"":false,"~/":null}]},7,null,{"child":"collision"}]`)
		}
		return response(`{"big":9007199254740993,"ok":true}`)
	}
	root := call("root", "collection", "/root", call("child", "single", "/child"))
	v1 := run(&fakeFetcher{handle: fixture}, root)
	runner, store, durable := v2TestRunner(&fakeFetcher{handle: fixture}, root)
	if err := runner.Run(context.Background(), "id"); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(v2Replay(durable.chunks))
	if string(raw) != string(v1.Data) || store.completion.Status != v1.Status || store.completion.Report.ErrorSummary.Total != v1.Report.ErrorSummary.Total {
		t.Fatalf("v2 %s %+v; v1 %s %+v", raw, store.completion, v1.Data, v1.Report)
	}
}
func TestV2ResumeDoesNotFetchConfirmedPageAgain(t *testing.T) {
	for _, after := range []bool{false, true} {
		t.Run(map[bool]string{false: "before_commit", true: "after_commit"}[after], func(t *testing.T) {
			fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
				if strings.HasSuffix(r.Request.URL, "/root") {
					return response(`[{"id":1},{"id":2}]`)
				}
				return response(`true`)
			}}
			runner, store, durable := v2TestRunner(fake, call("root", "collection", "/root", call("child", "single", "/{parent.id}")))
			durable.crashAt = 3
			durable.afterCommit = after
			if err := runner.Run(context.Background(), "id"); err == nil || store.finished {
				t.Fatal("expected recoverable interruption", err)
			}
			durable.crashAt = 0
			store.current.Status = execution.Pending
			if err := runner.Run(context.Background(), "id"); err != nil {
				t.Fatal(err)
			}
			if fake.count("https://provider.test/root") != 1 || store.completion.Status != execution.Completed {
				t.Fatal(fake.calls, store.completion)
			}
			got, _ := json.Marshal(v2Replay(durable.chunks))
			if string(got) != `{"root":[{"child":true,"id":1},{"child":true,"id":2}]}` {
				t.Fatal(string(got))
			}
		})
	}
}
func TestV2PaginationKeepsAbsolutePositionsAndAncestorBindings(t *testing.T) {
	fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		switch {
		case strings.Contains(r.Request.URL, "page=1"):
			return response(`[1,2]`)
		case strings.Contains(r.Request.URL, "page=2"):
			return response(`[3]`)
		default:
			return response(`true`)
		}
	}}
	root := call("root", "collection", "/root", call("child", "single", "/{parent.value}"))
	root.Pagination = &definition.Pagination{Page: "page", Start: 1, Pages: definition.PageSelection{All: true}}
	runner, store, durable := v2TestRunner(fake, root)
	if err := runner.Run(context.Background(), "id"); err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(v2Replay(durable.chunks))
	if string(got) != `{"root":[{"child":true,"value":1},{"child":true,"value":2},{"child":true,"value":3}]}` || store.completion.Status != execution.Completed {
		t.Fatal(string(got), store.completion)
	}
}
func TestV2ServiceCancellationLeavesJobResumable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) {
		cancel()
		return response(`true`)
	}}
	runner, store, _ := v2TestRunner(fake, call("root", "single", "/root"))
	if err := runner.Run(ctx, "id"); err == nil || store.finished {
		t.Fatal(err, store.finished)
	}
}
func TestV2PublishesParentBeforeChildFetch(t *testing.T) {
	var durable *v2MemoryStore
	fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		if strings.HasSuffix(r.Request.URL, "/root") {
			return response(`{"id":1}`)
		}
		if got := v2Replay(durable.chunks); !reflect.DeepEqual(got, map[string]any{"root": map[string]any{"id": json.Number("1")}}) {
			t.Fatalf("parent unavailable before child request: %#v", got)
		}
		return response(`true`)
	}}
	runner, _, storage := v2TestRunner(fake, call("root", "single", "/root", call("child", "single", "/child")))
	durable = storage
	if err := runner.Run(context.Background(), "id"); err != nil {
		t.Fatal(err)
	}
}

func TestV2FailureClosuresDoNotContaminateLaterRoots(t *testing.T) {
	fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		if strings.HasSuffix(r.Request.URL, "/bad") {
			return fetch.FetchResult{}, &fetch.UpstreamFailure{Code: "UPSTREAM_NOT_FOUND"}
		}
		return response(`{"nested":{"ok":true}}`)
	}}
	runner, store, durable := v2TestRunner(fake, call("bad", "single", "/bad"), call("good", "single", "/good"))
	if err := runner.Run(context.Background(), "id"); err != nil {
		t.Fatal(err)
	}
	if store.completion.Status != execution.CompletedWithErrors {
		t.Fatal(store.completion)
	}
	for _, chunk := range durable.chunks {
		if chunk.Path == "/bad" && chunk.Operation == "value" && chunk.Status != "failed" {
			t.Fatal("failed placeholder falsely completed", chunk)
		}
		if strings.HasPrefix(chunk.Path, "/good") && chunk.Operation == "close" && chunk.Status != "completed" {
			t.Fatal("independent success marked incomplete", chunk)
		}
	}
}

func TestV2AlreadyRunningJobIsNotClaimedTwice(t *testing.T) {
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) {
		t.Fatal("duplicate worker fetched")
		return response(`true`)
	}}
	runner, store, _ := v2TestRunner(fake, call("root", "single", "/root"))
	store.current.Status = execution.Running
	if err := runner.Run(context.Background(), "id"); err != nil || store.finished {
		t.Fatal(err)
	}
}

func TestV2IncompatibleCheckpointFailsWithoutHTTP(t *testing.T) {
	fake := &fakeFetcher{handle: func(context.Context, fetch.FetchRequest) (fetch.FetchResult, error) {
		t.Fatal("incompatible checkpoint fetched")
		return response(`true`)
	}}
	runner, store, durable := v2TestRunner(fake, call("root", "single", "/root"))
	durable.state.CheckpointVersion = 99
	if err := runner.Run(context.Background(), "id"); err != nil {
		t.Fatal(err)
	}
	if store.completion.Error == nil || store.completion.Error.Code != "UNSUPPORTED_CHECKPOINT_VERSION" {
		t.Fatal(store.completion)
	}
}

func TestV2CrashMatrixPreservesExactlyOncePublication(t *testing.T) {
	fixture := func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		if strings.HasSuffix(r.Request.URL, "/root") {
			return response(`[{"id":1},{"id":2}]`)
		}
		return response(`{"value":true}`)
	}
	root := call("root", "collection", "/root", call("child", "single", "/{parent.id}"))
	baseline, _, baseStore := v2TestRunner(&fakeFetcher{handle: fixture}, root)
	if err := baseline.Run(context.Background(), "id"); err != nil {
		t.Fatal(err)
	}
	expected, _ := json.Marshal(v2Replay(baseStore.chunks))
	for crash := 1; crash <= baseStore.commits; crash++ {
		for _, after := range []bool{false, true} {
			runner, store, durable := v2TestRunner(&fakeFetcher{handle: fixture}, root)
			durable.crashAt = crash
			durable.afterCommit = after
			if err := runner.Run(context.Background(), "id"); err == nil {
				t.Fatalf("crash %d after %v was not injected", crash, after)
			}
			durable.crashAt = 0
			store.current.Status = execution.Pending
			if err := runner.Run(context.Background(), "id"); err != nil {
				t.Fatalf("crash %d after %v resume: %v", crash, after, err)
			}
			got, _ := json.Marshal(v2Replay(durable.chunks))
			if string(got) != string(expected) || len(durable.chunks) != len(baseStore.chunks) {
				t.Fatalf("crash %d after %v changed result/publications", crash, after)
			}
		}
	}
}
