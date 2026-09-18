package config

type SecurityConfig struct {
	AuthenticationEnabled bool             `json:"authenticationEnabled"`
	PrincipalsFile        string           `json:"principalsFile"`
	IdentityKeyFile       string           `json:"identityKeyFile"`
	Encryption            EncryptionConfig `json:"encryption"`
	Egress                EgressConfig     `json:"egress"`
}
