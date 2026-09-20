package domain

type NodeWebhookSummary struct {
	Pending   int64 `json:"pending"`
	Delivered int64 `json:"delivered"`
	Failed    int64 `json:"failed"`
}
