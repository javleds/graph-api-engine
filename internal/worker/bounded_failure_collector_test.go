package worker

import (
	"context"
	"encoding/json"
	"fmt"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

func TestCollectorDeterministicNumericOrderAndExactCount(t *testing.T) {
	for iteration := 0; iteration < 10; iteration++ {
		collector := NewBoundedFailureCollector(3, 10000, 128)
		var group sync.WaitGroup
		for _, index := range rand.New(rand.NewSource(int64(iteration))).Perm(30) {
			index := index
			group.Add(1)
			go func() {
				defer group.Done()
				path := fmt.Sprintf("root[%d]", index)
				collector.RecordAt(execution.NodeFailure{Code: "TYPE_MISMATCH", Scope: "node", Message: "Tipo inválido.", NodePath: &path}, []int{0, index})
			}()
		}
		group.Wait()
		collector.Record(execution.NodeFailure{Code: "EXECUTION_TIMEOUT", Scope: "execution"})
		report := collector.Snapshot()
		if report.ErrorSummary.Total != 31 || report.ErrorSummary.Returned != 3 || !report.ErrorSummary.Truncated {
			t.Fatal(report)
		}
		if report.Errors[0].Scope != "execution" || *report.Errors[1].NodePath != "root[0]" || *report.Errors[2].NodePath != "root[1]" {
			t.Fatal(report.Errors)
		}
		*report.Errors[1].NodePath = "modified"
		if *collector.Snapshot().Errors[1].NodePath != "root[0]" {
			t.Fatal("snapshot aliases collector")
		}
	}
}
func TestCollectorByteBudgetDoesNotLoseTotal(t *testing.T) {
	collector := NewBoundedFailureCollector(10, 2, 10)
	collector.Record(execution.NodeFailure{Code: "TYPE_MISMATCH", Message: "Un mensaje muy largo"})
	report := collector.Snapshot()
	if report.ErrorSummary.Total != 1 || report.ErrorSummary.Returned != 0 || !report.ErrorSummary.Truncated {
		t.Fatal(report)
	}
}
func TestDiagnosticLimitDoesNotStopWork(t *testing.T) {
	fake := &fakeFetcher{handle: func(_ context.Context, r fetch.FetchRequest) (fetch.FetchResult, error) {
		if strings.HasSuffix(r.Request.URL, "/root") {
			return response(`[1,2,3,4]`)
		}
		return fetch.FetchResult{}, &fetch.UpstreamFailure{Code: "UPSTREAM_NOT_FOUND", Attempts: 1}
	}}
	engine := Engine{Fetcher: fake, Options: Options{MaxDetails: 1}}
	done := engine.Execute(context.Background(), compiled(call("root", "collection", "/root", call("child", "single", "/{parent.value}"))), "owner", "id")
	if done.Report.ErrorSummary.Total != 4 || done.Report.ErrorSummary.Returned != 1 || done.Status != execution.CompletedWithErrors || len(fake.calls) != 5 {
		t.Fatal(done, fake.calls)
	}
}

func TestMessageTruncationPreservesUTF8WithIndicator(t *testing.T) {
	collector := NewBoundedFailureCollector(1, 10000, 10)
	collector.Record(execution.NodeFailure{Message: "áéíóú mensaje"})
	message := collector.Snapshot().Errors[0].Message
	if len(message) > 10 || !utf8.ValidString(message) || !strings.HasSuffix(message, "…") {
		t.Fatalf("invalid truncation %q", message)
	}
}
func TestReportBudgetIncludesSummaryAndRoots(t *testing.T) {
	collector := NewBoundedFailureCollector(10, 600, 200)
	for index := 0; index < 5; index++ {
		collector.RecordAt(execution.NodeFailure{Code: "TYPE_MISMATCH", Message: "Tipo incorrecto."}, []int{index})
	}
	report := collector.SnapshotWithRoots([]execution.RootOutcome{{Name: strings.Repeat("x", 64), Status: execution.CompletedWithErrors, DataAvailable: true}})
	encoded, _ := json.Marshal(report)
	if len(encoded) > 600 || report.ErrorSummary.Total != 5 || !report.ErrorSummary.Truncated {
		t.Fatal(len(encoded), report)
	}
}
