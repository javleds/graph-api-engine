package config

type SecurityConfig struct {
	AuthenticationEnabled bool             `json:"authenticationEnabled" yaml:"authenticationEnabled"`
	PrincipalsFile        string           `json:"principalsFile" yaml:"principalsFile"`
	IdentityKeyFile       string           `json:"identityKeyFile" yaml:"identityKeyFile"`
	Encryption            EncryptionConfig `json:"encryption" yaml:"encryption"`
	Playground            PlaygroundConfig `json:"playground" yaml:"playground"`
	Egress                EgressConfig     `json:"egress" yaml:"egress"`
}
