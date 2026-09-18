package domain

type QuotaGroup struct {
	Name           string
	Origins        []string
	CredentialMode string
	Limits         QuotaRule
}
