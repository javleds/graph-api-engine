package dto

type WebhookStatusResponse struct {
	Status   string `json:"status"`
	Attempts int    `json:"attempts"`
}
