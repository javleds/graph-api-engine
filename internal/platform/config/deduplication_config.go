package config

type DeduplicationConfig struct {
	Driver           string   `json:"driver"`
	MaxActiveFlights int      `json:"maxActiveFlights"`
	MaxOperationTime Duration `json:"maxOperationTime"`
}
