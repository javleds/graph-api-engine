package dto

import "encoding/json"

type PageSelectionDTO struct {
	All   bool
	Count int64
}

func (p *PageSelectionDTO) UnmarshalJSON(data []byte) error {
	if string(data) == `"all"` {
		p.All = true
		return nil
	}
	return json.Unmarshal(data, &p.Count)
}
func (p PageSelectionDTO) MarshalJSON() ([]byte, error) {
	if p.All || p.Count == 0 {
		return []byte(`"all"`), nil
	}
	return json.Marshal(p.Count)
}
