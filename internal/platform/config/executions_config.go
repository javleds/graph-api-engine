package config

type ExecutionsConfig struct {
	V2                    V2Config     `json:"v2" yaml:"v2"`
	MaxConcurrent         int          `json:"maxConcurrent" yaml:"maxConcurrent" validate:"gt=0"`
	Timeout               Duration     `json:"timeout" yaml:"timeout" validate:"gt=0"`
	MaxDefinitionDepth    int          `json:"maxDefinitionDepth" yaml:"maxDefinitionDepth" validate:"gt=0"`
	MaxDefinitionNodes    int          `json:"maxDefinitionNodes" yaml:"maxDefinitionNodes" validate:"gt=0"`
	MaxChildrenPerNode    int          `json:"maxChildrenPerNode" yaml:"maxChildrenPerNode" validate:"gt=0"`
	MaxRootCalls          int          `json:"maxRootCalls" yaml:"maxRootCalls" validate:"gt=0"`
	MaxBranchGoroutines   int          `json:"maxBranchGoroutines" yaml:"maxBranchGoroutines" validate:"gt=0"`
	MaxPagesPerCall       int          `json:"maxPagesPerCall" yaml:"maxPagesPerCall" validate:"gt=0"`
	MaxLogicalRequests    int          `json:"maxLogicalRequests" yaml:"maxLogicalRequests" validate:"gt=0"`
	MaxHTTPAttempts       int          `json:"maxHTTPAttempts" yaml:"maxHTTPAttempts" validate:"gt=0"`
	MaxResultNodes        int          `json:"maxResultNodes" yaml:"maxResultNodes" validate:"gt=0"`
	MaxResultBytes        int          `json:"maxResultBytes" yaml:"maxResultBytes" validate:"gt=0"`
	DetectRepeatedPages   bool         `json:"detectRepeatedPages" yaml:"detectRepeatedPages"`
	ProgressFlushInterval Duration     `json:"progressFlushInterval" yaml:"progressFlushInterval" validate:"gt=0"`
	Errors                ErrorsConfig `json:"errors" yaml:"errors"`
}
