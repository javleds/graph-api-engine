package infrastructure

type PrincipalRecord struct {
	OwnerID     string   `json:"ownerId"`
	TokenSHA256 string   `json:"tokenSha256"`
	Enabled     bool     `json:"enabled"`
	Permissions []string `json:"permissions"`
}
