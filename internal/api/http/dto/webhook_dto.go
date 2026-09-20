package dto

type WebhookDTO struct {
	URL            string            `json:"url"`
	Method         string            `json:"method"`
	Headers        map[string]string `json:"headers"`
	Auth           AuthenticationDTO `json:"auth"`
	Payload        string            `json:"payload"`
	Ordering       string            `json:"ordering,omitempty"`
	MaxInFlight    int               `json:"maxInFlight,omitempty"`
	MaxInFlightSet bool              `json:"-"`
}
