package config

type MaintenanceConfig struct {
	Interval  Duration `json:"interval" yaml:"interval" validate:"gt=0"`
	BatchSize int      `json:"batchSize" yaml:"batchSize" validate:"gt=0"`
}
