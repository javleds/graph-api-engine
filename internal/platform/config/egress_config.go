package config

type EgressConfig struct {
	UpstreamAllowedOrigins       []string `json:"upstreamAllowedOrigins"`
	WebhookAllowedOrigins        []string `json:"webhookAllowedOrigins"`
	AllowedPrivateCIDRs          []string `json:"allowedPrivateCIDRs"`
	AllowHTTPForDevelopment      bool     `json:"allowHTTPForDevelopment"`
}
