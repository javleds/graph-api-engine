package config

type ErrorsConfig struct {
	MaxDetails      int `json:"maxDetails"`
	MaxReportBytes  int `json:"maxReportBytes"`
	MaxMessageBytes int `json:"maxMessageBytes"`
}
