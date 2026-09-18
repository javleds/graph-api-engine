package http

import "time"

type TransportOptions struct {
	MaxResponseBytes          int64
	ConnectTimeout            time.Duration
	TLSHandshakeTimeout       time.Duration
	ResponseHeaderTimeout     time.Duration
	MaxIdleConnections        int
	MaxIdleConnectionsPerHost int
	IdleConnectionTimeout     time.Duration
}
