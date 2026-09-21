package infrastructure

type playgroundSessionClaims struct {
	Version int   `json:"v"`
	Expires int64 `json:"exp"`
}
