package http

import (
	"bytes"
	"context"
	"encoding/json"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	execution "github.com/javleds/api-engine/internal/execution/domain"
	executionstore "github.com/javleds/api-engine/internal/execution/infrastructure/sqlite"
	notification "github.com/javleds/api-engine/internal/notification/application"
	outboxstore "github.com/javleds/api-engine/internal/notification/infrastructure/sqlite"
	dbplatform "github.com/javleds/api-engine/internal/platform/sqlite"
	security "github.com/javleds/api-engine/internal/security/infrastructure"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestLostAcknowledgementRetriesDurableEventWithFreshSignature(t *testing.T) {
	received := make(chan capturedRequest, 2)
	var attempts atomic.Int32
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- capturedRequest{headers: r.Header.Clone(), body: string(body)}
		if attempts.Add(1) == 1 {
			connection, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = connection.Close()
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer receiver.Close()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "engine.db")
	db, err := dbplatform.Open(ctx, path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err = dbplatform.Migrate(ctx, db, "engine"); err != nil {
		t.Fatal(err)
	}
	protector, err := security.NewAESGCMProtector("test", map[string][]byte{"test": bytes.Repeat([]byte{3}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	store := executionstore.NewExecutionStore(db, protector)
	now := time.Now().UTC().Truncate(time.Millisecond)
	target := definition.Webhook{URL: receiver.URL, Method: "POST", Payload: "status", Auth: definition.Authentication{Type: "hmac", Secret: "shared-secret"}}
	raw, _ := json.Marshal(definition.CompiledDefinition{Version: 1, Webhook: &target})
	snapshot := execution.ExecutionSnapshot{ID: "execution", OwnerID: "owner", Definition: raw, WebhookEnabled: true, CreatedAt: now, MetadataExpiresAt: now.Add(time.Hour)}
	if _, _, err = store.Create(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	completion := execution.ExecutionCompletion{ID: snapshot.ID, Status: execution.Failed, Error: &execution.ExecutionFailure{Code: "PROCESS_INTERRUPTED", Message: "El proceso se interrumpió."}, Report: execution.EmptyReport(), FinishedAt: now}
	builder := notification.WebhookPayloadBuilder{PublicBaseURL: "https://engine.test"}
	if err = builder.PrepareCompletion(ctx, snapshot, &completion); err != nil {
		t.Fatal(err)
	}
	if err = executionstore.NewExecutionFinalizer(store).Finish(ctx, completion); err != nil {
		t.Fatal(err)
	}
	sender := localSender(t, receiver)
	sender.Clock = fixedClock{now}
	outbox := outboxstore.NewSQLiteWebhookOutbox(db, protector)
	options := notification.DispatcherOptions{InstanceID: "first-process", MaxConcurrent: 1, BaseDelay: time.Second, MaxBackoff: time.Second, Jitter: func(delay time.Duration) time.Duration { return delay }}
	dispatcher := notification.WebhookDispatcher{Outbox: outbox, Sender: sender, Clock: fixedClock{now}, Options: options}
	if err = dispatcher.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	status, err := outbox.Status(ctx, "owner", snapshot.ID)
	if err != nil || status.Status != "retrying" || status.Attempts != 1 {
		t.Fatal(status, err)
	}
	// Reopen storage before retry: identity and payload must survive the lost ACK.
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = dbplatform.Open(ctx, path, false)
	if err != nil {
		t.Fatal(err)
	}
	outbox = outboxstore.NewSQLiteWebhookOutbox(db, protector)
	later := now.Add(2 * time.Second)
	sender.Clock = fixedClock{later}
	options.InstanceID = "next-process"
	dispatcher = notification.WebhookDispatcher{Outbox: outbox, Sender: sender, Clock: fixedClock{later}, Options: options}
	if err = dispatcher.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	status, err = outbox.Status(ctx, "owner", snapshot.ID)
	if err != nil || status.Status != "delivered" || status.Attempts != 2 {
		t.Fatal(status, err)
	}
	first, second := <-received, <-received
	if first.body != second.body || first.headers.Get("X-Engine-Event-Id") != second.headers.Get("X-Engine-Event-Id") || first.headers.Get("X-Engine-Signature") == second.headers.Get("X-Engine-Signature") || first.headers.Get("X-Engine-Timestamp") == second.headers.Get("X-Engine-Timestamp") {
		t.Fatal("retry identity/body changed or signature was reused", first, second)
	}
	if second.headers.Get("X-Engine-Signature") != Sign("shared-secret", completion.Delivery.EventID, second.headers.Get("X-Engine-Timestamp"), []byte(second.body)) {
		t.Fatal("retry signature does not match persisted body")
	}
	_, persisted, err := executionstore.NewExecutionStore(db, protector).Load(ctx, snapshot.ID)
	if err != nil || persisted.Status != execution.Failed || persisted.Error.Code != "PROCESS_INTERRUPTED" {
		t.Fatal("delivery changed execution outcome", persisted, err)
	}
}
