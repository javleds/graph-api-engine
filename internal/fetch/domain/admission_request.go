package domain

import "time"

type AdmissionRequest struct {
	CooldownScopes []string
	Rules          []QuotaRule
	ExecutionID    string
	Deadline       time.Time
	ProviderScope  string
}
