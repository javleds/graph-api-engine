package config

type QuotaLimits struct {
	Requests    int      `json:"requests"`
	Per         Duration `json:"per"`
	Concurrency int      `json:"concurrency"`
}
