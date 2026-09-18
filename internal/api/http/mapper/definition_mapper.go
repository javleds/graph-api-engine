package mapper

import (
	"fmt"

	"github.com/javleds/api-engine/internal/api/http/dto"
	"github.com/javleds/api-engine/internal/definition/application"
	"github.com/javleds/api-engine/internal/definition/domain"
)

func Compile(input dto.CreateExecutionRequest, limits application.ValidationLimits) (*domain.CompiledDefinition, error) {
	api := input.Definition.API
	rate, err := mapRate(api.Limits)
	if err != nil {
		return nil, err
	}
	cache, err := mapCache(api.Cache, domain.CachePolicy{})
	if err != nil {
		return nil, err
	}
	result := domain.CompiledDefinition{Version: input.Definition.Version, API: domain.APIConnection{BaseURL: api.BaseURL, Auth: mapAuth(api.Auth), Headers: mapHeaders(api.Headers), Limits: rate, Cache: cache}, Roots: []domain.CompiledCall{}}
	for i, call := range input.Definition.Calls.Calls {
		pointer := "/definition/calls"
		if input.Definition.Calls.WasArray {
			pointer += fmt.Sprintf("/%d", i)
		}
		compiled, err := mapCall(call, pointer, i, cache)
		if err != nil {
			return nil, err
		}
		result.Roots = append(result.Roots, compiled)
	}
	if w := input.Webhook; w != nil {
		result.Webhook = &domain.Webhook{URL: w.URL, Method: w.Method, Headers: mapHeaders(w.Headers), Auth: mapAuth(w.Auth), Payload: w.Payload}
	}
	return application.Compile(result, limits)
}
func mapCall(call dto.CallDTO, pointer string, root int, inherited domain.CachePolicy) (domain.CompiledCall, error) {
	result := domain.CompiledCall{Name: call.Name, Type: call.Type, Pointer: pointer, RootIndex: root, Children: []domain.CompiledCall{}, Request: domain.RequestTemplate{Method: call.Request.Method, Path: domain.Template{Source: call.Request.Path}, Query: map[string]any{}, Headers: mapHeaders(call.Request.Headers)}}
	for key, value := range call.Request.Query {
		result.Request.Query[key] = value
	}
	var err error
	result.Cache, err = mapCache(call.Cache, inherited)
	if err != nil {
		return result, err
	}
	if call.Select != nil {
		result.Select, err = domain.ParseDotPath(*call.Select)
		if err != nil {
			return result, err
		}
	}
	if call.Limits != nil {
		rate, err := mapRate(*call.Limits)
		if err != nil {
			return result, err
		}
		result.Limits = &rate
	}
	if p := call.Pagination; p != nil {
		start := int64(1)
		if p.Start != nil {
			start = *p.Start
		}
		result.Pagination = &domain.Pagination{Page: p.Page, PageSize: p.PageSize, Start: start, Pages: domain.PageSelection{All: p.Pages.All || p.Pages.Count == 0, Count: p.Pages.Count}}
	}
	for i, child := range call.Children {
		compiled, err := mapCall(child, fmt.Sprintf("%s/children/%d", pointer, i), root, result.Cache)
		if err != nil {
			return result, err
		}
		result.Children = append(result.Children, compiled)
	}
	return result, nil
}
func mapCache(cache *dto.CachePolicyDTO, inherited domain.CachePolicy) (domain.CachePolicy, error) {
	if cache == nil {
		return inherited, nil
	}
	if !cache.Enabled {
		return domain.CachePolicy{}, nil
	}
	ttl, err := domain.ParseDuration(cache.TTL)
	if err != nil {
		return domain.CachePolicy{}, domain.Invalid("/cache/ttl", "invalid duration")
	}
	return domain.CachePolicy{Enabled: true, TTL: ttl}, nil
}
func mapRate(rate dto.LimitsDTO) (domain.RatePolicy, error) {
	result := domain.RatePolicy{Requests: rate.Requests, Concurrency: rate.Concurrency}
	if rate.Per != "" {
		duration, err := domain.ParseDuration(rate.Per)
		if err != nil {
			return result, domain.Invalid("/limits/per", "invalid duration")
		}
		result.Per = duration
	}
	return result, nil
}
func mapAuth(auth dto.AuthenticationDTO) domain.Authentication {
	kind := auth.Type
	if kind == "" {
		kind = "none"
	}
	return domain.Authentication{Type: kind, Token: auth.Token, Username: auth.Username, Password: auth.Password, Name: auth.Name, Value: auth.Value, Secret: auth.Secret}
}
func mapHeaders(headers map[string]string) map[string]string {
	result := map[string]string{}
	for key, value := range headers {
		result[key] = value
	}
	return result
}
