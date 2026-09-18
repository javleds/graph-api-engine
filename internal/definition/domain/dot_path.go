package domain

import (
	"strconv"
	"strings"
)

func ParseDotPath(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	if strings.ContainsAny(path, "[]*$(){}") {
		return nil, Invalid("", "unsupported selector")
	}
	parts := strings.Split(path, ".")
	for _, part := range parts {
		if part == "" {
			return nil, Invalid("", "empty selector segment")
		}
	}
	return parts, nil
}
func Select(value any, path []string) (any, error) {
	current := value
	for _, segment := range path {
		switch object := current.(type) {
		case map[string]any:
			next, ok := object[segment]
			if !ok {
				return nil, selectionError()
			}
			current = next
		case []any:
			if segment == "" || strings.IndexFunc(segment, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
				return nil, selectionError()
			}
			index, err := strconv.ParseUint(segment, 10, 64)
			if err != nil || index >= uint64(len(object)) {
				return nil, selectionError()
			}
			current = object[index]
		default:
			return nil, selectionError()
		}
	}
	return current, nil
}
func selectionError() error {
	return &ValidationError{Code: "SELECT_PATH_NOT_FOUND", Message: "selected property is absent"}
}
