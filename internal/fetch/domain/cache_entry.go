package domain

import "time"

type CacheEntry struct {
	Key           string
	Response      RawResponse
	FetchedAt     time.Time
	ExpiresAt     time.Time
	InitialAge    time.Duration
	SchemaVersion int
}

func (e CacheEntry) Clone() CacheEntry { e.Response = e.Response.Clone(); return e }
