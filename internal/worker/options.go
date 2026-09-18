package worker

import "time"

// Options bounds work and memory separately; zero fields use service defaults.
type Options struct {
	MaxBranchGoroutines   int
	MaxPagesPerCall       int
	MaxLogicalRequests    uint64
	MaxHTTPAttempts       uint64
	MaxResultNodes        uint64
	MaxResultBytes        int
	MaxDetails            int
	MaxReportBytes        int
	MaxMessageBytes       int
	DetectRepeatedPages   bool
	Timeout               time.Duration
	PersistenceTimeout    time.Duration
	ProgressFlushInterval time.Duration
}

func DefaultOptions() Options {
	return Options{MaxBranchGoroutines: 16, MaxPagesPerCall: 1000, MaxLogicalRequests: 50000, MaxHTTPAttempts: 60000, MaxResultNodes: 100000, MaxResultBytes: 16777216, MaxDetails: 1000, MaxReportBytes: 1048576, MaxMessageBytes: 512, DetectRepeatedPages: true, Timeout: 6 * time.Hour, PersistenceTimeout: 5 * time.Second, ProgressFlushInterval: time.Second}
}
func (o Options) defaults() Options {
	d := DefaultOptions()
	if o.MaxBranchGoroutines <= 0 {
		o.MaxBranchGoroutines = d.MaxBranchGoroutines
	}
	if o.MaxPagesPerCall <= 0 {
		o.MaxPagesPerCall = d.MaxPagesPerCall
	}
	if o.MaxLogicalRequests == 0 {
		o.MaxLogicalRequests = d.MaxLogicalRequests
	}
	if o.MaxHTTPAttempts == 0 {
		o.MaxHTTPAttempts = d.MaxHTTPAttempts
	}
	if o.MaxResultNodes == 0 {
		o.MaxResultNodes = d.MaxResultNodes
	}
	if o.MaxResultBytes <= 0 {
		o.MaxResultBytes = d.MaxResultBytes
	}
	if o.MaxDetails <= 0 {
		o.MaxDetails = d.MaxDetails
	}
	if o.MaxReportBytes <= 0 {
		o.MaxReportBytes = d.MaxReportBytes
	}
	if o.MaxMessageBytes <= 0 {
		o.MaxMessageBytes = d.MaxMessageBytes
	}
	if o.Timeout <= 0 {
		o.Timeout = d.Timeout
	}
	if o.PersistenceTimeout <= 0 {
		o.PersistenceTimeout = d.PersistenceTimeout
	}
	if o.ProgressFlushInterval <= 0 {
		o.ProgressFlushInterval = d.ProgressFlushInterval
	}
	return o
}
