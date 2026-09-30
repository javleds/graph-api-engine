package application

import (
	"encoding/json"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func CacheEntryFor(key string, response domain.RawResponse, ttl, maxTTL time.Duration, ignoreServerCachePolicy bool, now time.Time) (domain.CacheEntry, bool) {
	if response.StatusCode != 200 || !json.Valid(response.Body) || ttl <= 0 {
		return domain.CacheEntry{}, false
	}
	headers := http.Header(response.Headers)
	if len(headers.Values("Set-Cookie")) > 0 {
		return domain.CacheEntry{}, false
	}
	for _, value := range strings.Split(strings.Join(headers.Values("Vary"), ","), ",") {
		if strings.TrimSpace(value) == "*" {
			return domain.CacheEntry{}, false
		}
	}
	if maxTTL > 0 && ttl > maxTTL {
		ttl = maxTTL
	}
	age := time.Duration(0)
	if !ignoreServerCachePolicy {
		if seconds, err := strconv.ParseInt(headers.Get("Age"), 10, 64); err == nil && seconds >= 0 {
			if seconds > int64((1<<63-1)/int64(time.Second)) {
				return domain.CacheEntry{}, false
			}
			age = time.Duration(seconds) * time.Second
		}
		if date, err := http.ParseTime(headers.Get("Date")); err == nil && now.Sub(date) > age {
			age = now.Sub(date)
		}
		for _, directive := range strings.Split(strings.Join(headers.Values("Cache-Control"), ","), ",") {
			pair := strings.SplitN(strings.TrimSpace(directive), "=", 2)
			name := strings.ToLower(pair[0])
			if name == "no-store" || name == "no-cache" {
				return domain.CacheEntry{}, false
			}
			if name == "max-age" {
				if len(pair) != 2 {
					return domain.CacheEntry{}, false
				}
				seconds, err := strconv.ParseInt(strings.Trim(pair[1], `"`), 10, 64)
				if err != nil || seconds < 0 || seconds > int64((1<<63-1)/int64(time.Second)) {
					return domain.CacheEntry{}, false
				}
				maximum := time.Duration(seconds) * time.Second
				if maximum < ttl {
					ttl = maximum
				}
			}
		}
	}
	remaining := ttl - age
	if remaining <= 0 {
		return domain.CacheEntry{}, false
	}
	expires := now.Add(remaining)
	if !ignoreServerCachePolicy {
		if explicit, err := http.ParseTime(headers.Get("Expires")); err == nil && explicit.Before(expires) {
			expires = explicit
		}
	}
	if !expires.After(now) {
		return domain.CacheEntry{}, false
	}
	return domain.CacheEntry{Key: key, Response: response.Clone(), FetchedAt: now, ExpiresAt: expires, InitialAge: age, SchemaVersion: 1}, true
}

func cacheFresh(entry domain.CacheEntry, ttl time.Duration, now time.Time) bool {
	return entry.SchemaVersion == 1 && now.Before(entry.ExpiresAt) && entry.InitialAge+now.Sub(entry.FetchedAt) < ttl && entry.Response.StatusCode == 200 && json.Valid(entry.Response.Body)
}
