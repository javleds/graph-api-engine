package config

type QuotaLimits struct {
	Requests    int      `json:"requests" yaml:"requests" validate:"gt=0"`
	Per         Duration `json:"per" yaml:"per" validate:"gt=0"`
	Concurrency int      `json:"concurrency" yaml:"concurrency" validate:"gte=0"`
}
