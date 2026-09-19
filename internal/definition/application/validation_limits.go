package application

import "time"

type ValidationLimits struct {
	AllowAnyOriginForDevelopment bool
	MaxDefinitionDepth           int
	MaxDefinitionNodes           int
	MaxChildrenPerNode           int
	MaxRootCalls                 int
	HTTPConcurrency              int
	MaxTTL                       time.Duration
	AllowHTTPForDevelopment      bool
	UpstreamAllowedOrigins       []string
	WebhookAllowedOrigins        []string
}

func DefaultValidationLimits() ValidationLimits {
	return ValidationLimits{MaxDefinitionDepth: 8, MaxDefinitionNodes: 128, MaxChildrenPerNode: 16, MaxRootCalls: 16, HTTPConcurrency: 20, MaxTTL: 30 * 24 * time.Hour}
}
