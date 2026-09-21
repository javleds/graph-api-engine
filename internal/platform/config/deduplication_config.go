package config

type DeduplicationConfig struct {
	Driver           string   `json:"driver" yaml:"driver"`
	MaxActiveFlights int      `json:"maxActiveFlights" yaml:"maxActiveFlights" validate:"gt=0"`
	MaxOperationTime Duration `json:"maxOperationTime" yaml:"maxOperationTime" validate:"gt=0"`
}
