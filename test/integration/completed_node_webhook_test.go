package integration

import "encoding/json"

type completedNodeWebhook struct {
	Type    string          `json:"type"`
	EventID string          `json:"eventId"`
	Status  string          `json:"status"`
	Result  json.RawMessage `json:"result"`
}
