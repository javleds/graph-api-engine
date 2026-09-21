package config

type QuotaGroup struct {
	Name           string      `json:"name" yaml:"name"`
	Origins        []string    `json:"origins" yaml:"origins"`
	CredentialMode string      `json:"credentialMode" yaml:"credentialMode"`
	Limits         QuotaLimits `json:"limits" yaml:"limits"`
}
