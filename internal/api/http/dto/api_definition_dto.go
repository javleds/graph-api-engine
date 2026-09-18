package dto

type APIDefinitionDTO struct {
	BaseURL string            `json:"baseUrl"`
	Auth    AuthenticationDTO `json:"auth"`
	Headers map[string]string `json:"headers"`
	Limits  LimitsDTO         `json:"limits"`
	Cache   *CachePolicyDTO   `json:"cache"`
}
