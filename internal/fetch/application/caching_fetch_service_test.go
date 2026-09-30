package application

import (
	"context"
	"errors"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"net/http"
	"testing"
	"time"
)

func TestCacheBypassIsolationAndDegradation(t *testing.T) {
	calls := 0
	fetcher := NewFetchService(transportStub(func(context.Context, domain.OutboundRequest) (domain.RawResponse, error) {
		calls++
		return domain.RawResponse{StatusCode: 200, Body: []byte(`{"value":1}`)}, nil
	}))
	cache := &memoryCache{}
	service := NewCachingFetchService(fetcher, cache, []byte("stable-secret"))
	input := domain.FetchRequest{OwnerID: "a", Request: domain.OutboundRequest{Method: "GET", URL: "https://example.com/data"}, Policy: domain.FetchPolicy{CacheEnabled: true, CacheTTL: time.Hour}}
	_, err := service.Fetch(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Fetch(context.Background(), input)
	if err != nil || !result.CacheHit || calls != 1 {
		t.Fatalf("%+v %v calls %d", result, err, calls)
	}
	input.OwnerID = "b"
	_, _ = service.Fetch(context.Background(), input)
	if calls != 2 {
		t.Fatal("cache crossed owners")
	}
	gets, puts := cache.gets, cache.puts
	input.Policy.CacheEnabled = false
	_, _ = service.Fetch(context.Background(), input)
	if cache.gets != gets || cache.puts != puts || calls != 3 {
		t.Fatal("disabled cache accessed")
	}
	input.Policy.CacheEnabled = true
	cache.err = errors.New("SQL unavailable")
	_, err = service.Fetch(context.Background(), input)
	if err != nil || calls != 4 {
		t.Fatalf("cache error failed fetch %v", err)
	}
}

func TestCacheHTTPFreshness(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	response := domain.RawResponse{StatusCode: 200, Body: []byte(`[]`)}
	for _, headers := range []map[string][]string{{"Cache-Control": {"no-store"}}, {"Cache-Control": {"no-cache"}}, {"Vary": {"*"}}, {"Set-Cookie": {"secret"}}} {
		response.Headers = headers
		if _, ok := CacheEntryFor("key", response, time.Hour, time.Hour, false, now); ok {
			t.Fatalf("cached prohibited response %v", headers)
		}
	}
	response.Headers = map[string][]string{"Age": {"10"}, "Cache-Control": {"max-age=20"}}
	entry, ok := CacheEntryFor("key", response, time.Hour, time.Hour, false, now)
	if !ok || entry.ExpiresAt.Sub(now) != 10*time.Second {
		t.Fatalf("%+v", entry)
	}
	if cacheFresh(entry, 5*time.Second, now) {
		t.Fatal("short TTL ignored initial age")
	}
	if cacheFresh(entry, time.Hour, entry.ExpiresAt) {
		t.Fatal("exact expiration accepted")
	}
}

func TestIgnoreServerCachePolicyUsesRequestTTLAndKeepsSafetyChecks(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	response := domain.RawResponse{StatusCode: 200, Body: []byte(`[]`), Headers: map[string][]string{
		"Cache-Control": {"no-store, no-cache, max-age=0"},
		"Age":           {"3600"},
		"Expires":       {now.Add(-time.Hour).Format(http.TimeFormat)},
	}}
	entry, ok := CacheEntryFor("key", response, time.Hour, 30*time.Minute, true, now)
	if !ok || entry.ExpiresAt.Sub(now) != 30*time.Minute || entry.InitialAge != 0 {
		t.Fatalf("override did not use request TTL: %+v, %v", entry, ok)
	}
	response.Headers["Set-Cookie"] = []string{"session=secret"}
	if _, ok := CacheEntryFor("key", response, time.Hour, time.Hour, true, now); ok {
		t.Fatal("cached response with Set-Cookie")
	}
	delete(response.Headers, "Set-Cookie")
	response.Headers["Vary"] = []string{"*"}
	if _, ok := CacheEntryFor("key", response, time.Hour, time.Hour, true, now); ok {
		t.Fatal("cached response with Vary: *")
	}
}

func TestCacheOverrideDoesNotLeakIntoDefaultPolicy(t *testing.T) {
	calls := 0
	fetcher := NewFetchService(transportStub(func(context.Context, domain.OutboundRequest) (domain.RawResponse, error) {
		calls++
		return domain.RawResponse{StatusCode: 200, Body: []byte(`[]`), Headers: map[string][]string{"Cache-Control": {"no-store"}}}, nil
	}))
	service := NewCachingFetchService(fetcher, &memoryCache{}, []byte("key"))
	input := domain.FetchRequest{OwnerID: "owner", Request: domain.OutboundRequest{Method: "GET", URL: "https://example.com/data"}, Policy: domain.FetchPolicy{CacheEnabled: true, CacheTTL: time.Hour}}
	for i := 0; i < 2; i++ {
		result, err := service.Fetch(context.Background(), input)
		if err != nil || result.CacheHit {
			t.Fatalf("default policy reused prohibited response: %+v %v", result, err)
		}
	}
	input.Policy.IgnoreServerCachePolicy = true
	for i := 0; i < 2; i++ {
		result, err := service.Fetch(context.Background(), input)
		if err != nil || result.CacheHit != (i == 1) {
			t.Fatalf("override cache behavior: %+v %v", result, err)
		}
	}
	input.Policy.IgnoreServerCachePolicy = false
	result, err := service.Fetch(context.Background(), input)
	if err != nil || result.CacheHit || calls != 4 {
		t.Fatalf("override leaked into default policy: %+v %v calls=%d", result, err, calls)
	}
}

func TestRequestIdentityCanonicalizationAndIsolation(t *testing.T) {
	a := domain.FetchRequest{OwnerID: "owner", Request: domain.OutboundRequest{Method: "GET", URL: "https://EXAMPLE.com:443/a?b=2&a=1", Headers: map[string][]string{"x-test": {"value"}}}}
	b := a
	b.Request.URL = "https://example.com/a?a=1&b=2"
	b.Request.Headers = map[string][]string{"X-Test": {"value"}}
	first, _ := RequestKey([]byte("key"), a)
	second, _ := RequestKey([]byte("key"), b)
	if first != second {
		t.Fatal("equivalent identity split")
	}
	b.OwnerID = "other"
	second, _ = RequestKey([]byte("key"), b)
	if first == second {
		t.Fatal("owner isolation lost")
	}
	b = a
	b.Policy.CacheEnabled = true
	if FlightKey([]byte("key"), first, a) == FlightKey([]byte("key"), first, b) {
		t.Fatal("incompatible cache policies shared flight")
	}
}

func TestLeaderCacheRecheckReportsHitWithoutPhysicalAttempt(t *testing.T) {
	input := domain.FetchRequest{OwnerID: "owner", Request: domain.OutboundRequest{Method: "GET", URL: "https://example.com/items"}, Policy: domain.FetchPolicy{CacheEnabled: true, CacheTTL: time.Hour}}
	key, err := RequestKey([]byte("secret"), input)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	cache := &memoryCache{missFirst: true, entries: map[string]domain.CacheEntry{key: {Key: key, SchemaVersion: 1, FetchedAt: now, ExpiresAt: now.Add(time.Hour), Response: domain.RawResponse{StatusCode: 200, Body: []byte(`[]`), Attempts: 4}}}}
	raw := NewFetchService(transportStub(func(context.Context, domain.OutboundRequest) (domain.RawResponse, error) {
		t.Error("cache recheck performed HTTP")
		return domain.RawResponse{}, nil
	}))
	service := NewCachingFetchService(raw, cache, []byte("secret"))
	service.Flights = inlineFlight{}
	result, err := service.Fetch(context.Background(), input)
	if err != nil || !result.CacheHit || result.ReusedInFlight || result.Response.Attempts != 0 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}
