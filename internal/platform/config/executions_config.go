package config

type ExecutionsConfig struct {
	MaxConcurrent         int          `json:"maxConcurrent"`
	Timeout               Duration     `json:"timeout"`
	MaxDefinitionDepth    int          `json:"maxDefinitionDepth"`
	MaxDefinitionNodes    int          `json:"maxDefinitionNodes"`
	MaxChildrenPerNode    int          `json:"maxChildrenPerNode"`
	MaxRootCalls          int          `json:"maxRootCalls"`
	MaxBranchGoroutines   int          `json:"maxBranchGoroutines"`
	MaxPagesPerCall       int          `json:"maxPagesPerCall"`
	MaxLogicalRequests    int          `json:"maxLogicalRequests"`
	MaxHTTPAttempts       int          `json:"maxHTTPAttempts"`
	MaxResultNodes        int          `json:"maxResultNodes"`
	MaxResultBytes        int          `json:"maxResultBytes"`
	DetectRepeatedPages   bool         `json:"detectRepeatedPages"`
	ProgressFlushInterval Duration     `json:"progressFlushInterval"`
	Errors                ErrorsConfig `json:"errors"`
}
