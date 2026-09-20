package domain

type Webhook struct {
	URL         string
	Method      string
	Headers     map[string]string
	Auth        Authentication
	Payload     string
	Ordering    string `json:",omitempty"`
	MaxInFlight int    `json:",omitempty"`
}
