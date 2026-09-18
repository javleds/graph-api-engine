package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/javleds/api-engine/internal/bootstrap"
	"github.com/javleds/api-engine/internal/platform/config"
)

// I01: simultaneous accepted requests converge on the durable SQLite identity.
func TestConcurrentAcceptanceCreatesOneExecution(t *testing.T) {
	var requests atomic.Int32
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		select {
		case <-release:
			io.WriteString(w, `[]`)
		case <-r.Context().Done():
		}
	}))
	defer upstream.Close()
	cfg := fixtureConfig(t, upstream.URL, upstream.URL)
	const clients = 12
	cfg.Executions.MaxConcurrent = clients
	cfg.Fetch.AttemptTimeout = config.Duration(2 * time.Second)
	fixture := openFixture(t, cfg)
	body := definitionBody(upstream.URL, nil)
	statuses := make([]int, clients)
	responses := make([][]byte, clients)
	failures := make([]error, clients)
	start := make(chan struct{})
	var group sync.WaitGroup
	for index := range clients {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			request, err := http.NewRequest("POST", fixture.server.URL+"/v1/executions", bytes.NewReader(body))
			if err != nil {
				failures[index] = err
				return
			}
			request.Header.Set("Authorization", "Bearer "+fixture.token)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", "concurrent-identity")
			response, err := fixture.server.Client().Do(request)
			if err != nil {
				failures[index] = err
				return
			}
			defer response.Body.Close()
			statuses[index] = response.StatusCode
			responses[index], failures[index] = io.ReadAll(response.Body)
		}()
	}
	close(start)
	group.Wait()
	close(release)
	var winner string
	for index := range clients {
		if failures[index] != nil {
			t.Fatal(failures[index])
		}
		if statuses[index] != 202 {
			t.Fatalf("client %d: %d %s", index, statuses[index], responses[index])
		}
		var response map[string]any
		if err := json.Unmarshal(responses[index], &response); err != nil {
			t.Fatal(err)
		}
		id, _ := response["id"].(string)
		if id == "" {
			t.Fatalf("missing identity: %s", responses[index])
		}
		if winner == "" {
			winner = id
		}
		if id != winner {
			t.Fatalf("multiple execution identities: %s and %s", winner, id)
		}
	}
	if terminal := fixture.terminal(t, winner); terminal["status"] != "completed" {
		t.Fatal(terminal)
	}
	var count int
	if err := fixture.app.EngineDB.QueryRow("SELECT count(*) FROM executions").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 || requests.Load() != 1 {
		t.Fatalf("durable jobs=%d physical requests=%d", count, requests.Load())
	}
}

// I04: disconnect while the acceptance response is still blocked on the wire.
func TestClientDisconnectAfterCommitDoesNotCancelExecution(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	upstreamCanceled := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-release:
			io.WriteString(w, `[]`)
		case <-r.Context().Done():
			upstreamCanceled <- struct{}{}
		}
	}))
	defer upstream.Close()
	cfg := fixtureConfig(t, upstream.URL, upstream.URL)
	cfg.Fetch.AttemptTimeout = config.Duration(2 * time.Second)
	fixture := openFixture(t, cfg)
	committed := make(chan []byte, 1)
	disconnected := make(chan struct{})
	// Buffer only the acceptance response to control the disconnect boundary.
	delayedResponse := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := httptest.NewRecorder()
		fixture.app.HTTP.Handler.ServeHTTP(response, r)
		committed <- append([]byte(nil), response.Body.Bytes()...)
		<-r.Context().Done()
		close(disconnected)
	}))
	defer delayedResponse.Close()
	endpoint, err := url.Parse(delayedResponse.URL)
	if err != nil {
		t.Fatal(err)
	}
	connection, err := net.DialTimeout("tcp", endpoint.Host, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	request, err := http.NewRequest("POST", delayedResponse.URL+"/v1/executions", bytes.NewReader(definitionBody(upstream.URL, nil)))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+fixture.token)
	request.Header.Set("Content-Type", "application/json")
	if err := request.Write(connection); err != nil {
		t.Fatal(err)
	}
	var accepted []byte
	select {
	case accepted = <-committed:
	case <-time.After(5 * time.Second):
		t.Fatal("execution was not committed")
	}
	var response map[string]any
	if err := json.Unmarshal(accepted, &response); err != nil {
		t.Fatal(err)
	}
	id, _ := response["id"].(string)
	if id == "" {
		t.Fatalf("acceptance failed: %s", accepted)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("upstream request did not start")
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-disconnected:
	case <-time.After(time.Second):
		t.Fatal("server did not observe client disconnect")
	}
	select {
	case <-upstreamCanceled:
		t.Fatal("client disconnect canceled upstream work")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if terminal := fixture.terminal(t, id); terminal["status"] != "completed" {
		t.Fatalf("detached execution failed: %#v", terminal)
	}
}

// I18: configured disk reserve prevents acceptance without filling the disk.
func TestLowDiskRejectsBeforeDurableAcceptance(t *testing.T) {
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); io.WriteString(w, `[]`) }))
	defer upstream.Close()
	cfg := fixtureConfig(t, upstream.URL, upstream.URL)
	var filesystem syscall.Statfs_t
	if err := syscall.Statfs(filepath.Dir(cfg.Storage.Path), &filesystem); err != nil {
		t.Fatal(err)
	}
	available := uint64(filesystem.Bavail) * uint64(filesystem.Bsize)
	if available >= uint64(^uint(0)>>1) {
		t.Skip("available disk exceeds configurable signed integer range")
	}
	cfg.Storage.MinimumFreeDiskBytes = int(^uint(0) >> 1)
	fixture := openFixture(t, cfg)
	status, body := fixture.request(t, "POST", "/v1/executions", fixture.token, "low-disk", definitionBody(upstream.URL, nil))
	if status != 503 || !bytes.Contains(body, []byte(`"ENGINE_BUSY"`)) {
		t.Fatalf("low disk: %d %s", status, body)
	}
	var count int
	if err := fixture.app.EngineDB.QueryRow("SELECT count(*) FROM executions").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 || requests.Load() != 0 {
		t.Fatalf("rejected job persisted=%d upstream requests=%d", count, requests.Load())
	}
}

func TestUnavailableCriticalDatabaseBlocksStartupAndReadiness(t *testing.T) {
	cfg := fixtureConfig(t, "http://127.0.0.1:12345", "http://127.0.0.1:12345")
	if err := os.Mkdir(cfg.Storage.Path, 0700); err != nil {
		t.Fatal(err)
	}
	app, err := bootstrap.Build(context.Background(), cfg)
	if err == nil {
		app.Close()
		t.Fatal("startup accepted an unavailable critical database")
	}
	cfg = fixtureConfig(t, "http://127.0.0.1:12345", "http://127.0.0.1:12345")
	fixture := openFixture(t, cfg)
	if err := fixture.app.EngineDB.Close(); err != nil {
		t.Fatal(err)
	}
	status, body := fixture.request(t, "GET", "/health/ready", "", "", nil)
	if status != 503 {
		t.Fatalf("closed critical database ready: %d %s", status, body)
	}
}
