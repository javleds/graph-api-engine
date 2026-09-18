package domain

type RequestIdentity struct {
	OwnerID            string
	CredentialIdentity string
	Method             string
	URL                string
	Headers            map[string][]string
	BodyHash           string
}
