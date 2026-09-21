package config

type CacheConfig struct {
	Driver             string   `json:"driver" yaml:"driver"`
	Path               string   `json:"path" yaml:"path"`
	BusyTimeout        Duration `json:"busyTimeout" yaml:"busyTimeout" validate:"gt=0"`
	MaxOpenConnections int      `json:"maxOpenConnections" yaml:"maxOpenConnections" validate:"gt=0"`
	Synchronous        string   `json:"synchronous" yaml:"synchronous"`
	MaxEntryBytes      int      `json:"maxEntryBytes" yaml:"maxEntryBytes" validate:"gt=0"`
	MaxEntries         int      `json:"maxEntries" yaml:"maxEntries" validate:"gt=0"`
	MaxLogicalBytes    int      `json:"maxLogicalBytes" yaml:"maxLogicalBytes" validate:"gt=0"`
	MaxTTL             Duration `json:"maxTTL" yaml:"maxTTL" validate:"gt=0"`
}
