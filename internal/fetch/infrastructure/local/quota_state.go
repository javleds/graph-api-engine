package local

import (
	"github.com/javleds/api-engine/internal/fetch/domain"
	"time"
)

type quotaState struct {
	references       int
	rule             domain.QuotaRule
	next             time.Time
	cooldown         time.Time
	active           int
	lastUsed         time.Time
	initialization   *quotaInitialization
	pendingWrites    int
	persistenceError error
}
