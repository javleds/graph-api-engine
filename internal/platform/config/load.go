package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed defaults.yml
var defaultsYAML []byte

// Default returns structural defaults; deployment secrets, origins and public URL remain required.
func Default() Config {
	var result Config
	if err := decodeYAMLInto(defaultsYAML, nil, &result); err != nil {
		panic("decode embedded configuration defaults: " + err.Error())
	}
	return result
}
func Load(path string) (*Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return nil, err
	}
	if path == "" {
		path = os.Getenv("ENGINE_CONFIG")
	}
	if path == "" {
		return nil, fmt.Errorf("configuration file is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}
	if extension := filepath.Ext(path); extension != ".yml" && extension != ".yaml" {
		return nil, fmt.Errorf("configuration must use a .yml or .yaml file")
	}
	result, err := decodeYAML(data, os.LookupEnv)
	if err != nil {
		return nil, err
	}
	applyEnvironment(result, os.LookupEnv)
	if err := result.Validate(); err != nil {
		return nil, err
	}
	return result, nil
}
func Parse(data []byte) (*Config, error) {
	result, err := decodeYAML(data, os.LookupEnv)
	if err != nil {
		return nil, err
	}
	if err := result.Validate(); err != nil {
		return nil, err
	}
	return result, nil
}
func applyEnvironment(c *Config, lookup func(string) (string, bool)) {
	if value, ok := lookup("ENGINE_HTTP_ADDRESS"); ok {
		c.HTTP.Address = value
	}
	if value, ok := lookup("ENGINE_HTTP_PUBLIC_BASE_URL"); ok {
		c.HTTP.PublicBaseURL = value
	}
	if value, ok := lookup("ENGINE_LOG_LEVEL"); ok {
		c.Observability.LogLevel = value
	}
}
