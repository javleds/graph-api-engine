package config

type SecurityConfig struct {
	AuthenticationEnabled bool             `json:"authenticationEnabled"`
	PrincipalsFile        string           `json:"principalsFile"`
	IdentityKeyFile       string           `json:"identityKeyFile"`
	Encryption            EncryptionConfig `json:"encryption"`
	Playground            PlaygroundConfig `json:"playground"`
	Egress                EgressConfig     `json:"egress"`
}
