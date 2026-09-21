package config

type StorageConfig struct {
	Driver               string   `json:"driver" yaml:"driver"`
	Path                 string   `json:"path" yaml:"path"`
	BusyTimeout          Duration `json:"busyTimeout" yaml:"busyTimeout" validate:"gt=0"`
	MaxOpenConnections   int      `json:"maxOpenConnections" yaml:"maxOpenConnections" validate:"gt=0"`
	Synchronous          string   `json:"synchronous" yaml:"synchronous"`
	MetadataRetention    Duration `json:"metadataRetention" yaml:"metadataRetention" validate:"gt=0"`
	ResultRetention      Duration `json:"resultRetention" yaml:"resultRetention" validate:"gt=0"`
	MinimumFreeDiskBytes int      `json:"minimumFreeDiskBytes" yaml:"minimumFreeDiskBytes" validate:"gt=0"`
}
