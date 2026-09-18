package domain

type APIConnection struct {
	BaseURL string
	Auth    Authentication
	Headers map[string]string
	Limits  RatePolicy
	Cache   CachePolicy
}
