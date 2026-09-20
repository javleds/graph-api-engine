package integration

import "encoding/json"

type completeNodeResult struct {
	EventID  string          `json:"eventId"`
	Data     json.RawMessage `json:"data"`
	Complete bool            `json:"complete"`
}
