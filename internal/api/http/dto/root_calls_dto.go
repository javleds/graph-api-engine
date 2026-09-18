package dto

import (
	"bytes"
	"encoding/json"
)

type RootCallsDTO struct {
	Calls    []CallDTO
	WasArray bool
}

func (r *RootCallsDTO) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	r.WasArray = len(bytes.TrimSpace(data)) > 0 && bytes.TrimSpace(data)[0] == '['
	if r.WasArray {
		return decoder.Decode(&r.Calls)
	}
	var call CallDTO
	if err := decoder.Decode(&call); err != nil {
		return err
	}
	r.Calls = []CallDTO{call}
	return nil
}
func (r RootCallsDTO) MarshalJSON() ([]byte, error) {
	if r.WasArray || len(r.Calls) != 1 {
		return json.Marshal(r.Calls)
	}
	return json.Marshal(r.Calls[0])
}
