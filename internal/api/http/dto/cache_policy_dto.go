package dto

import "encoding/json"

type CachePolicyDTO struct {
	Enabled                 bool
	TTL                     string
	IgnoreServerCachePolicy bool
}

func (c *CachePolicyDTO) UnmarshalJSON(data []byte) error {
	if string(data) == "false" {
		*c = CachePolicyDTO{}
		return nil
	}
	*c = CachePolicyDTO{}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	c.Enabled = true
	if raw, ok := value["ttl"]; ok {
		if err := json.Unmarshal(raw, &c.TTL); err != nil {
			return err
		}
	}
	if raw, ok := value["ignoreServerCachePolicy"]; ok {
		if err := json.Unmarshal(raw, &c.IgnoreServerCachePolicy); err != nil {
			return err
		}
	}
	return nil
}
func (c CachePolicyDTO) MarshalJSON() ([]byte, error) {
	if !c.Enabled {
		return []byte("false"), nil
	}
	value := map[string]any{"ttl": c.TTL}
	if c.IgnoreServerCachePolicy {
		value["ignoreServerCachePolicy"] = true
	}
	return json.Marshal(value)
}
