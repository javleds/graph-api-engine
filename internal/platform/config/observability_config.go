package config

type ObservabilityConfig struct {
	LogLevel       string `json:"logLevel"`
	MetricsEnabled bool   `json:"metricsEnabled"`
	MetricsPath    string `json:"metricsPath"`
}
