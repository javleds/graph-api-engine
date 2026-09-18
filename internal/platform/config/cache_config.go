package config

type CacheConfig struct {
	Driver             string   `json:"driver"`
	Path               string   `json:"path"`
	BusyTimeout        Duration `json:"busyTimeout"`
	MaxOpenConnections int      `json:"maxOpenConnections"`
	Synchronous        string   `json:"synchronous"`
	MaxEntryBytes      int      `json:"maxEntryBytes"`
	MaxEntries         int      `json:"maxEntries"`
	MaxLogicalBytes    int      `json:"maxLogicalBytes"`
	MaxTTL             Duration `json:"maxTTL"`
}
