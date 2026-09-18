package domain

type FetchPolicyIdentity struct {
	CooldownScopes []string
	RequestKey     string
	Policy         FetchPolicy
	Rules          []QuotaRule
	ProviderScope  string
}
