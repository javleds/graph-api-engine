package application

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

func RequestKey(secret []byte, input domain.FetchRequest) (string, error) {
	u, err := url.Parse(input.Request.URL)
	if err != nil {
		return "", err
	}
	u.Scheme = strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		u.Host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		u.Host = "[" + host + "]"
	} else {
		u.Host = host
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", err
	}
	u.RawQuery = values.Encode()
	headers := make(map[string][]string)
	keys := make([]string, 0, len(input.Request.Headers))
	for key := range input.Request.Headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		name := http.CanonicalHeaderKey(key)
		headers[name] = append(headers[name], input.Request.Headers[key]...)
	}
	if len(headers["Accept"]) == 0 {
		headers["Accept"] = []string{"application/json"}
	}
	headers["User-Agent"] = []string{"api-execution-engine/1.1"}
	hash := sha256.Sum256(input.Request.Body)
	identity := domain.RequestIdentity{OwnerID: input.OwnerID, CredentialIdentity: input.CredentialIdentity, Method: strings.ToUpper(input.Request.Method), URL: u.String(), Headers: headers, BodyHash: hex.EncodeToString(hash[:]), IgnoreServerCachePolicy: input.Policy.IgnoreServerCachePolicy}
	data, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	return "http:v1:" + identityDigest(secret, data), nil
}

func FlightKey(secret []byte, requestKey string, input domain.FetchRequest) string {
	rules := append([]domain.QuotaRule(nil), input.Admission.Rules...)
	sort.Slice(rules, func(i, j int) bool { return rules[i].Scope < rules[j].Scope })
	identity := domain.FetchPolicyIdentity{RequestKey: requestKey, Policy: input.Policy.WithDefaults(), Rules: rules, ProviderScope: input.Admission.ProviderScope, CooldownScopes: input.Admission.CooldownScopes}
	data, _ := json.Marshal(identity)
	return "flight:v1:" + identityDigest(secret, data)
}

func identityDigest(secret, data []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}
