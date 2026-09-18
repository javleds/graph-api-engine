package bootstrap

import (
	"github.com/javleds/api-engine/internal/platform/config"
	"github.com/javleds/api-engine/internal/worker"
)

func workerOptions(cfg *config.Config) worker.Options {
	c := cfg.Executions
	return worker.Options{
		MaxBranchGoroutines:   c.MaxBranchGoroutines,
		MaxPagesPerCall:       c.MaxPagesPerCall,
		MaxLogicalRequests:    uint64(c.MaxLogicalRequests),
		MaxHTTPAttempts:       uint64(c.MaxHTTPAttempts),
		MaxResultNodes:        uint64(c.MaxResultNodes),
		MaxResultBytes:        c.MaxResultBytes,
		MaxDetails:            c.Errors.MaxDetails,
		MaxReportBytes:        c.Errors.MaxReportBytes,
		MaxMessageBytes:       c.Errors.MaxMessageBytes,
		DetectRepeatedPages:   c.DetectRepeatedPages,
		Timeout:               c.Timeout.Value(),
		PersistenceTimeout:    cfg.Shutdown.PersistenceTimeout.Value(),
		ProgressFlushInterval: c.ProgressFlushInterval.Value(),
	}
}
