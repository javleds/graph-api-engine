package application

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

func BuildRequestURL(base, path string, query map[string]string) (string, error) {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("INVALID_BASE_URL")
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.ContainsAny(path, "?#") {
		return "", errors.New("INVALID_REQUEST_PATH")
	}
	escaped := strings.TrimSuffix(u.EscapedPath(), "/") + path
	decoded, err := url.PathUnescape(escaped)
	if err != nil {
		return "", errors.New("INVALID_REQUEST_PATH")
	}
	for _, segment := range strings.Split(decoded, "/") {
		if segment == "." || segment == ".." || strings.Contains(segment, `\`) {
			return "", errors.New("INVALID_REQUEST_PATH")
		}
	}
	u.Path = decoded
	u.RawPath = escaped
	values := url.Values{}
	for k, v := range query {
		values.Set(k, v)
	}
	u.RawQuery = values.Encode()
	return u.String(), nil
}

func MergeHeaders(api, node map[string]string) map[string][]string {
	result := make(map[string][]string)
	for k, v := range api {
		result[http.CanonicalHeaderKey(k)] = []string{v}
	}
	for k, v := range node {
		result[http.CanonicalHeaderKey(k)] = []string{v}
	}
	if len(result["Accept"]) == 0 {
		result["Accept"] = []string{"application/json"}
	}
	result["User-Agent"] = []string{"api-execution-engine/1.1"}
	return result
}
