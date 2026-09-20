package domain

type WebhookDeliveryPage struct {
	Deliveries []WebhookDeliveryStatus `json:"deliveries"`
	NextCursor string                  `json:"nextCursor"`
	HasMore    bool                    `json:"hasMore"`
}
