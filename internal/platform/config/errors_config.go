package config

type ErrorsConfig struct {
	MaxDetails      int `json:"maxDetails" yaml:"maxDetails" validate:"gt=0"`
	MaxReportBytes  int `json:"maxReportBytes" yaml:"maxReportBytes" validate:"gt=0"`
	MaxMessageBytes int `json:"maxMessageBytes" yaml:"maxMessageBytes" validate:"gt=0"`
}
