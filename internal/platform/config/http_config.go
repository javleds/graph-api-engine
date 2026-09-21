package config

type HTTPConfig struct {
	Address           string   `json:"address" yaml:"address"`
	PublicBaseURL     string   `json:"publicBaseUrl" yaml:"publicBaseUrl"`
	ReadHeaderTimeout Duration `json:"readHeaderTimeout" yaml:"readHeaderTimeout" validate:"gt=0"`
	ReadTimeout       Duration `json:"readTimeout" yaml:"readTimeout" validate:"gt=0"`
	WriteTimeout      Duration `json:"writeTimeout" yaml:"writeTimeout" validate:"gt=0"`
	IdleTimeout       Duration `json:"idleTimeout" yaml:"idleTimeout" validate:"gt=0"`
	MaxRequestBytes   int      `json:"maxRequestBytes" yaml:"maxRequestBytes" validate:"gt=0"`
}
