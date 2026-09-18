package infrastructure

type PrincipalsFile struct {
	Version    int               `json:"version"`
	Principals []PrincipalRecord `json:"principals"`
}
