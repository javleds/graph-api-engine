package application

import (
	notification "github.com/javleds/api-engine/internal/notification/domain"
	"time"
)

type savedAttempt struct {
	delivery     notification.WebhookDelivery
	success      bool
	next         *time.Time
	status       int
	code         string
	contextError error
}
