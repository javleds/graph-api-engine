package config

type MaintenanceConfig struct {
	Interval  Duration `json:"interval"`
	BatchSize int      `json:"batchSize"`
}
