package mapper

import (
	"encoding/json"

	"github.com/javleds/api-engine/internal/definition/domain"
)

// FingerprintDefinition retains pre-policy identity for the default V2 request.
// The execution snapshot itself must keep its normalized delivery policy.
func FingerprintDefinition(definition domain.CompiledDefinition, apiVersion int) ([]byte, error) {
	if webhook := definition.Webhook; apiVersion == 2 && webhook != nil && webhook.Ordering == "completion" && webhook.MaxInFlight == 1 {
		copy := *webhook
		copy.Ordering = ""
		copy.MaxInFlight = 0
		definition.Webhook = &copy
	}
	return json.Marshal(definition)
}
