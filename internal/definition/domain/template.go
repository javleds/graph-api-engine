package domain

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

type Template struct {
	Source     string
	References []TemplateReference
}

func CompileTemplate(source string, ancestors []string) (Template, error) {
	result := Template{Source: source}
	for cursor := 0; cursor < len(source); {
		next := strings.IndexAny(source[cursor:], "{}")
		if next < 0 {
			break
		}
		start := cursor + next
		if source[start] != '{' {
			return result, Invalid("", "orphan template delimiter")
		}
		closing := strings.IndexByte(source[start+1:], '}')
		if closing < 0 {
			return result, Invalid("", "unclosed template")
		}
		end := start + closing + 1
		parts := strings.SplitN(source[start+1:end], ".", 2)
		if len(parts) != 2 || parts[1] == "" || strings.ContainsAny(parts[0], "{}") {
			return result, Invalid("", "invalid template reference")
		}
		matches := 0
		for _, ancestor := range ancestors {
			if ancestor == parts[0] {
				matches++
			}
		}
		if parts[0] == "parent" && len(ancestors) > 0 {
			matches = 1
		}
		if matches != 1 {
			return result, Invalid("", "reference must identify one ancestor")
		}
		path, err := ParseDotPath(parts[1])
		if err != nil {
			return result, err
		}
		result.References = append(result.References, TemplateReference{Start: start, End: end + 1, Context: parts[0], Path: path})
		cursor = end + 1
	}
	return result, nil
}
func (t Template) Resolve(contexts map[string]any, path bool) (string, error) {
	var result strings.Builder
	cursor := 0
	for _, ref := range t.References {
		result.WriteString(t.Source[cursor:ref.Start])
		value, ok := contexts[ref.Context]
		if !ok {
			return "", bindingError()
		}
		value, err := Select(value, ref.Path)
		if err != nil {
			return "", bindingError()
		}
		literal, err := ScalarString(value)
		if err != nil {
			return "", err
		}
		if strings.IndexFunc(literal, unicode.IsControl) >= 0 {
			return "", Invalid("", "control in binding")
		}
		if path {
			if literal == "." || literal == ".." {
				return "", Invalid("", "unsafe path binding")
			}
			literal = url.PathEscape(literal)
		}
		result.WriteString(literal)
		cursor = ref.End
	}
	result.WriteString(t.Source[cursor:])
	return result.String(), nil
}
func ScalarString(value any) (string, error) {
	switch scalar := value.(type) {
	case string:
		return scalar, nil
	case json.Number:
		return scalar.String(), nil
	case bool:
		return strconv.FormatBool(scalar), nil
	case int:
		return strconv.Itoa(scalar), nil
	case int64:
		return strconv.FormatInt(scalar, 10), nil
	case float64:
		return strconv.FormatFloat(scalar, 'g', -1, 64), nil
	}
	return "", &ValidationError{Code: "TYPE_MISMATCH", Message: fmt.Sprintf("URL binding requires scalar, received %T", value)}
}
func bindingError() error {
	return &ValidationError{Code: "BINDING_PATH_NOT_FOUND", Message: "required ancestor property is absent"}
}
