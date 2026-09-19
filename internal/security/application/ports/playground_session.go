package ports

type PlaygroundSession interface {
	PrincipalAuthenticator
	Login(username, password string) (string, error)
}
