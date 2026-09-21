package config

type ThrottleConfig struct {
	Driver         string       `json:"driver" yaml:"driver"`
	Utilization    float64      `json:"utilization" yaml:"utilization" validate:"gt=0"`
	MaxQuotaScopes int          `json:"maxQuotaScopes" yaml:"maxQuotaScopes" validate:"gt=0"`
	ColdStart      string       `json:"coldStart" yaml:"coldStart"`
	SafetyMargin   Duration     `json:"safetyMargin" yaml:"safetyMargin" validate:"gt=0"`
	QuotaGroups    []QuotaGroup `json:"quotaGroups" yaml:"quotaGroups"`
}
