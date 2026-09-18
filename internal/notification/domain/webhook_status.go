package domain

type WebhookStatus struct {
	Status   string `json:"status"`
	Attempts int    `json:"attempts"`
}
