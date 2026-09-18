package config

type QuotaGroup struct {
	Name           string      `json:"name"`
	Origins        []string    `json:"origins"`
	CredentialMode string      `json:"credentialMode"`
	Limits         QuotaLimits `json:"limits"`
}
