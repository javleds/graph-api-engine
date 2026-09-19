package application

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/javleds/api-engine/internal/definition/domain"
)

func validateURL(raw string, base, allowHTTP, allowAnyOrigin bool, origins []string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || strings.Contains(raw, "#") || (!allowHTTP && u.Scheme != "https") || (u.Scheme != "https" && u.Scheme != "http") {
		return domain.Invalid("/url", "destination must be an absolute permitted HTTP URL")
	}
	if base && (u.RawQuery != "" || u.ForceQuery) {
		return domain.Invalid("/definition/api/baseUrl", "base URL cannot contain query")
	}
	if strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return domain.Invalid("/url", "control character in URL")
	}
	if len(origins) > 0 && !(allowHTTP && allowAnyOrigin) {
		origin := strings.ToLower(u.Scheme + "://" + u.Host)
		found := false
		for _, allowed := range origins {
			if strings.EqualFold(strings.TrimRight(allowed, "/"), origin) {
				found = true
			}
		}
		if !found {
			return &domain.ValidationError{Code: "EGRESS_DENIED", Message: "destination origin is not allowed"}
		}
	}
	return nil
}
func validatePath(path string) error {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.ContainsAny(path, "?#\\") || strings.IndexFunc(path, unicode.IsControl) >= 0 {
		return fmt.Errorf("invalid request path")
	}
	for _, part := range strings.Split(path, "/") {
		decoded, err := url.PathUnescape(part)
		if err != nil {
			return fmt.Errorf("invalid path escape")
		}
		if decoded == "." || decoded == ".." || strings.ContainsAny(decoded, "\\") {
			return fmt.Errorf("path traversal is forbidden")
		}
	}
	return nil
}
func validateHeaders(headers map[string]string, auth domain.Authentication, webhook bool) error {
	seen := map[string]bool{}
	owned := ""
	switch auth.Type {
	case "bearer", "basic":
		owned = "authorization"
	case "header":
		owned = strings.ToLower(auth.Name)
	}
	for name, value := range headers {
		lower := strings.ToLower(name)
		if seen[lower] || reservedHeader(lower, webhook) || lower == owned || strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return domain.Invalid("/headers", "duplicate, reserved or invalid header")
		}
		seen[lower] = true
	}
	if auth.Type == "header" && reservedHeader(strings.ToLower(auth.Name), webhook) {
		return domain.Invalid("/auth/name", "reserved authentication header")
	}
	for _, secret := range []string{auth.Token, auth.Name, auth.Value, auth.Username, auth.Password} {
		if strings.IndexFunc(secret, unicode.IsControl) >= 0 {
			return domain.Invalid("/auth", "control character in authentication")
		}
	}
	return nil
}
func reservedHeader(name string, webhook bool) bool {
	switch name {
	case "host", "connection", "content-length", "transfer-encoding", "te", "trailer", "upgrade", "keep-alive", "proxy-authenticate", "proxy-authorization", "user-agent":
		return true
	}
	return webhook && (name == "content-type" || name == "x-engine-event-id" || name == "x-engine-timestamp" || name == "x-engine-signature")
}
func validateWebhook(w *domain.Webhook, limits ValidationLimits) error {
	if err := validateURL(w.URL, false, limits.AllowHTTPForDevelopment, limits.AllowAnyOriginForDevelopment, limits.WebhookAllowedOrigins); err != nil {
		return err
	}
	if w.Method == "" {
		w.Method = "POST"
	}
	if w.Payload == "" {
		w.Payload = "status"
	}
	if w.Auth.Type == "" {
		w.Auth.Type = "none"
	}
	return validateHeaders(w.Headers, w.Auth, true)
}
