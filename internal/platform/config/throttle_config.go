package config

type ThrottleConfig struct {
	Driver         string       `json:"driver"`
	Utilization    float64      `json:"utilization"`
	MaxQuotaScopes int          `json:"maxQuotaScopes"`
	ColdStart      string       `json:"coldStart"`
	SafetyMargin   Duration     `json:"safetyMargin"`
	QuotaGroups    []QuotaGroup `json:"quotaGroups"`
}
