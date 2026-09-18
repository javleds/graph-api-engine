package config

type FetchConfig struct {
	MaxConcurrentHTTPRequests int         `json:"maxConcurrentHTTPRequests"`
	ConnectTimeout            Duration    `json:"connectTimeout"`
	TlsHandshakeTimeout       Duration    `json:"tlsHandshakeTimeout"`
	ResponseHeaderTimeout     Duration    `json:"responseHeaderTimeout"`
	AttemptTimeout            Duration    `json:"attemptTimeout"`
	MaxResponseBytes          int         `json:"maxResponseBytes"`
	MaxJSONDepth              int         `json:"maxJSONDepth"`
	MaxJSONValuesPerResponse  int         `json:"maxJSONValuesPerResponse"`
	MaxIdleConnections        int         `json:"maxIdleConnections"`
	MaxIdleConnectionsPerHost int         `json:"maxIdleConnectionsPerHost"`
	IdleConnectionTimeout     Duration    `json:"idleConnectionTimeout"`
	Retry                     RetryConfig `json:"retry"`
}
