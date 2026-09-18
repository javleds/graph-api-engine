package http

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// Sign uses the exact persisted payload bytes, never reserialized JSON.
func Sign(secret, eventID, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(eventID + "." + timestamp + "."))
	_, _ = mac.Write(body)
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}
