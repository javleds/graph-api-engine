package config

type ObservabilityConfig struct {
	LogLevel       string `json:"logLevel" yaml:"logLevel"`
	MetricsEnabled bool   `json:"metricsEnabled" yaml:"metricsEnabled"`
	MetricsPath    string `json:"metricsPath" yaml:"metricsPath"`
}
