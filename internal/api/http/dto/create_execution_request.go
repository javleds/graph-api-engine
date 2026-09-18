package dto

type CreateExecutionRequest struct {
	Definition DefinitionDTO `json:"definition"`
	Webhook    *WebhookDTO   `json:"webhook,omitempty"`
}
