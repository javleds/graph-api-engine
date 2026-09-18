package dto

import "encoding/json"

type CachePolicyDTO struct {
	Enabled bool
	TTL     string
}

func (c *CachePolicyDTO) UnmarshalJSON(data []byte) error {
	if string(data) == "false" {
		*c = CachePolicyDTO{}
		return nil
	}
	var value map[string]string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	c.Enabled = true
	c.TTL = value["ttl"]
	return nil
}
func (c CachePolicyDTO) MarshalJSON() ([]byte, error) {
	if !c.Enabled {
		return []byte("false"), nil
	}
	return json.Marshal(map[string]string{"ttl": c.TTL})
}
