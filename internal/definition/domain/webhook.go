package domain

import "encoding/json"

type Webhook struct {
	URL            string
	Method         string
	Headers        map[string]string
	Auth           Authentication
	Payload        string
	Ordering       string `json:",omitempty"`
	MaxInFlight    int    `json:",omitempty"`
	MaxInFlightSet bool   `json:"-"`
}

func (w Webhook) MarshalJSON() ([]byte, error) {
	var inFlight *int
	if w.MaxInFlightSet || w.MaxInFlight != 0 {
		value := w.MaxInFlight
		inFlight = &value
	}
	return json.Marshal(webhookWire{w.URL, w.Method, w.Headers, w.Auth, w.Payload, w.Ordering, inFlight})
}

func (w *Webhook) UnmarshalJSON(data []byte) error {
	var value webhookWire
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	w.URL, w.Method, w.Headers, w.Auth, w.Payload, w.Ordering = value.URL, value.Method, value.Headers, value.Auth, value.Payload, value.Ordering
	w.MaxInFlight = 0
	w.MaxInFlightSet = value.MaxInFlight != nil
	if value.MaxInFlight != nil {
		w.MaxInFlight = *value.MaxInFlight
	}
	return nil
}
