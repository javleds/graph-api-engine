package config

type EgressConfig struct {
	AllowAnyOriginForDevelopment bool     `json:"allowAnyOriginForDevelopment"`
	UpstreamAllowedOrigins       []string `json:"upstreamAllowedOrigins"`
	WebhookAllowedOrigins        []string `json:"webhookAllowedOrigins"`
	AllowedPrivateCIDRs          []string `json:"allowedPrivateCIDRs"`
	AllowHTTPForDevelopment      bool     `json:"allowHTTPForDevelopment"`
}
