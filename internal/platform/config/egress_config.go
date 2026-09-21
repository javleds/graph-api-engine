package config

type EgressConfig struct {
	UpstreamAllowedOrigins  []string `json:"upstreamAllowedOrigins" yaml:"upstreamAllowedOrigins"`
	WebhookAllowedOrigins   []string `json:"webhookAllowedOrigins" yaml:"webhookAllowedOrigins"`
	AllowedPrivateCIDRs     []string `json:"allowedPrivateCIDRs" yaml:"allowedPrivateCIDRs"`
	AllowHTTPForDevelopment bool     `json:"allowHTTPForDevelopment" yaml:"allowHTTPForDevelopment"`
}
