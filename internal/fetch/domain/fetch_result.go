package domain

type FetchResult struct {
	Response       RawResponse
	CacheHit       bool
	ReusedInFlight bool
}
