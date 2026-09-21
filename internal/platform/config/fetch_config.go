package config

type FetchConfig struct {
	MaxConcurrentHTTPRequests int         `json:"maxConcurrentHTTPRequests" yaml:"maxConcurrentHTTPRequests" validate:"gt=0"`
	ConnectTimeout            Duration    `json:"connectTimeout" yaml:"connectTimeout" validate:"gt=0"`
	TlsHandshakeTimeout       Duration    `json:"tlsHandshakeTimeout" yaml:"tlsHandshakeTimeout" validate:"gt=0"`
	ResponseHeaderTimeout     Duration    `json:"responseHeaderTimeout" yaml:"responseHeaderTimeout" validate:"gt=0"`
	AttemptTimeout            Duration    `json:"attemptTimeout" yaml:"attemptTimeout" validate:"gt=0"`
	MaxResponseBytes          int         `json:"maxResponseBytes" yaml:"maxResponseBytes" validate:"gt=0"`
	MaxJSONDepth              int         `json:"maxJSONDepth" yaml:"maxJSONDepth" validate:"gt=0"`
	MaxJSONValuesPerResponse  int         `json:"maxJSONValuesPerResponse" yaml:"maxJSONValuesPerResponse" validate:"gt=0"`
	MaxIdleConnections        int         `json:"maxIdleConnections" yaml:"maxIdleConnections" validate:"gt=0"`
	MaxIdleConnectionsPerHost int         `json:"maxIdleConnectionsPerHost" yaml:"maxIdleConnectionsPerHost" validate:"gt=0"`
	IdleConnectionTimeout     Duration    `json:"idleConnectionTimeout" yaml:"idleConnectionTimeout" validate:"gt=0"`
	Retry                     RetryConfig `json:"retry" yaml:"retry"`
}
