package config

type StorageConfig struct {
	Driver               string   `json:"driver"`
	Path                 string   `json:"path"`
	BusyTimeout          Duration `json:"busyTimeout"`
	MaxOpenConnections   int      `json:"maxOpenConnections"`
	Synchronous          string   `json:"synchronous"`
	MetadataRetention    Duration `json:"metadataRetention"`
	ResultRetention      Duration `json:"resultRetention"`
	MinimumFreeDiskBytes int      `json:"minimumFreeDiskBytes"`
}
