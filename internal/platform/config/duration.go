package config

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/javleds/api-engine/internal/definition/domain"
	"gopkg.in/yaml.v3"
)

type Duration time.Duration

func (d *Duration) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	parsed, err := domain.ParseDuration(value)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
		return fmt.Errorf("duration must be a string")
	}
	parsed, err := domain.ParseDuration(value.Value)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}
func (d Duration) Value() time.Duration { return time.Duration(d) }
func (d Duration) MarshalJSON() ([]byte, error) {
	for _, unit := range []time.Duration{24 * time.Hour, time.Hour, time.Minute, time.Second, time.Millisecond} {
		if time.Duration(d)%unit == 0 {
			suffix := map[time.Duration]string{24 * time.Hour: "d", time.Hour: "h", time.Minute: "m", time.Second: "s", time.Millisecond: "ms"}[unit]
			return json.Marshal(fmt.Sprintf("%d%s", time.Duration(d)/unit, suffix))
		}
	}
	return nil, fmt.Errorf("duration must be an integer number of milliseconds")
}
