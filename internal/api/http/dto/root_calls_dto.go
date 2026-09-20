package dto

import (
	"bytes"
	"encoding/json"
)

type RootCallsDTO struct {
	Calls []CallDTO
}

func (r *RootCallsDTO) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode(&r.Calls)
}
func (r RootCallsDTO) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Calls)
}
