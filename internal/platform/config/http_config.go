package config

type HTTPConfig struct {
	Address           string   `json:"address"`
	PublicBaseURL     string   `json:"publicBaseUrl"`
	ReadHeaderTimeout Duration `json:"readHeaderTimeout"`
	ReadTimeout       Duration `json:"readTimeout"`
	WriteTimeout      Duration `json:"writeTimeout"`
	IdleTimeout       Duration `json:"idleTimeout"`
	MaxRequestBytes   int      `json:"maxRequestBytes"`
}
