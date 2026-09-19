package config

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/javleds/api-engine/internal/definition/infrastructure/schema"
)

//go:embed defaults.yml
var defaultsYAML []byte

// Default returns structural defaults; deployment secrets, origins and public URL remain required.
func Default() Config {
	canonical, err := normalizeYAML(defaultsYAML, nil)
	if err != nil {
		panic("invalid embedded configuration defaults: " + err.Error())
	}
	var result Config
	if err := json.Unmarshal(canonical, &result); err != nil {
		panic("decode embedded configuration defaults: " + err.Error())
	}
	return result
}
func Load(path string) (*Config, error) {
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
func decodeJSON(data []byte) (*Config, error) {
	value, err := schema.DecodeJSON(data, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid configuration JSON")
	}
	if _, ok := value.(map[string]any); !ok {
		return nil, fmt.Errorf("configuration must be an object")
	}
	if hasNull(value) {
		return nil, fmt.Errorf("configuration values cannot be null")
	}
	result := Default()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	return &result, nil
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
func hasNull(value any) bool {
	if value == nil {
		return true
	}
	switch object := value.(type) {
	case map[string]any:
		for _, v := range object {
			if hasNull(v) {
				return true
			}
		}
	case []any:
		for _, v := range object {
			if hasNull(v) {
				return true
			}
		}
	}
	return false
}
