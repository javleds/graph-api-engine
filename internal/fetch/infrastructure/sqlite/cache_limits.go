package sqlite

import "time"

type CacheLimits struct {
	MaxEntryBytes   int64
	MaxEntries      int64
	MaxLogicalBytes int64
	MaxTTL          time.Duration
}
