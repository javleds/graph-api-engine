package application

import (
	"encoding/json"
	"regexp"
	"strconv"

	"github.com/javleds/api-engine/internal/definition/domain"
)

func Compile(def domain.CompiledDefinition, limits ValidationLimits) (*domain.CompiledDefinition, error) {
	if def.Version != 1 {
		return nil, &domain.ValidationError{Code: "UNSUPPORTED_VERSION", Path: "/definition/version", Message: "version must be 1"}
	}
	if len(def.Roots) == 0 || len(def.Roots) > limits.MaxRootCalls {
		return nil, domain.Invalid("/definition/calls", "root count exceeds limits")
	}
	if err := validateURL(def.API.BaseURL, true, limits.AllowHTTPForDevelopment, limits.UpstreamAllowedOrigins); err != nil {
		return nil, err
	}
	if err := validateHeaders(def.API.Headers, def.API.Auth, false); err != nil {
		return nil, err
	}
	if err := validateRate(def.API.Limits, true); err != nil {
		return nil, err
	}
	if def.API.Limits.Concurrency == 0 {
		def.API.Limits.Concurrency = limits.HTTPConcurrency
	}
	if def.API.Auth.Type == "" {
		def.API.Auth.Type = "none"
	}
	if def.API.Cache.TTL > limits.MaxTTL {
		return nil, domain.Invalid("/definition/api/cache", "TTL exceeds server limit")
	}
	if def.Webhook != nil {
		if err := validateWebhook(def.Webhook, limits); err != nil {
			return nil, err
		}
	}
	count := 0
	if err := compileCalls(def.Roots, nil, 1, &count, limits, def.API.Auth); err != nil {
		return nil, err
	}
	return &def, nil
}
func compileCalls(calls []domain.CompiledCall, ancestors []string, depth int, count *int, limits ValidationLimits, auth domain.Authentication) error {
	names := make(map[string]bool)
	for i := range calls {
		call := &calls[i]
		*count++
		if depth > limits.MaxDefinitionDepth || *count > limits.MaxDefinitionNodes || len(call.Children) > limits.MaxChildrenPerNode {
			return domain.Invalid(call.Pointer, "definition exceeds structural limits")
		}
		if names[call.Name] || call.Name == "parent" || !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`).MatchString(call.Name) {
			return domain.Invalid(call.Pointer+"/name", "invalid or duplicate name")
		}
		names[call.Name] = true
		if call.Type != "single" && call.Type != "collection" {
			return domain.Invalid(call.Pointer+"/type", "unknown call type")
		}
		if call.Request.Method != "GET" {
			return &domain.ValidationError{Code: "UNSUPPORTED_METHOD", Path: call.Pointer + "/request/method", Message: "only GET is supported"}
		}
		if err := validatePath(call.Request.Path.Source); err != nil {
			return domain.Invalid(call.Pointer+"/request/path", err.Error())
		}
		template, err := domain.CompileTemplate(call.Request.Path.Source, ancestors)
		if err != nil {
			return domain.Invalid(call.Pointer+"/request/path", err.Error())
		}
		call.Request.Path = template
		for key, value := range call.Request.Query {
			if source, ok := value.(string); ok {
				if _, err := domain.CompileTemplate(source, ancestors); err != nil {
					return domain.Invalid(call.Pointer+"/request/query/"+key, "invalid ancestor reference")
				}
			}
		}
		if err := validateHeaders(call.Request.Headers, auth, false); err != nil {
			return err
		}
		if call.Limits != nil {
			if err := validateRate(*call.Limits, false); err != nil {
				return err
			}
		}
		if call.Cache.TTL > limits.MaxTTL {
			return domain.Invalid(call.Pointer+"/cache", "TTL exceeds server limit")
		}
		if err := validatePagination(*call); err != nil {
			return err
		}
		next := append(append([]string(nil), ancestors...), call.Name)
		if err := compileCalls(call.Children, next, depth+1, count, limits, auth); err != nil {
			return err
		}
	}
	return nil
}
func validateRate(rate domain.RatePolicy, required bool) error {
	if rate.Requests < 0 || rate.Per < 0 || rate.Concurrency < 0 || ((rate.Requests == 0) != (rate.Per == 0)) {
		return domain.Invalid("/limits", "invalid quota")
	}
	if required && rate.Requests == 0 {
		return domain.Invalid("/definition/api/limits", "API rate is required")
	}
	if !required && rate.Requests == 0 && rate.Concurrency == 0 {
		return domain.Invalid("/limits", "empty quota")
	}
	return nil
}
func validatePagination(call domain.CompiledCall) error {
	p := call.Pagination
	if p == nil {
		return nil
	}
	if call.Type != "collection" || p.Page == "" || p.Start < 0 || (!p.Pages.All && p.Pages.Count < 1) {
		return domain.Invalid(call.Pointer+"/pagination", "invalid pagination")
	}
	if _, exists := call.Request.Query[p.Page]; exists {
		return domain.Invalid(call.Pointer+"/pagination/page", "page is managed exclusively by pagination")
	}
	if p.PageSize == "" {
		return nil
	}
	if p.PageSize == p.Page {
		return domain.Invalid(call.Pointer+"/pagination", "page and pageSize must differ")
	}
	size, ok := call.Request.Query[p.PageSize].(json.Number)
	if !ok {
		return domain.Invalid(call.Pointer+"/pagination/pageSize", "page size must be a static JSON integer")
	}
	n, err := strconv.ParseInt(size.String(), 10, 64)
	if err != nil || n <= 0 {
		return domain.Invalid(call.Pointer+"/pagination/pageSize", "page size must be positive")
	}
	return nil
}
