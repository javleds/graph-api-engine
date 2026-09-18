package worker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	fetchapp "github.com/javleds/api-engine/internal/fetch/application"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (p *CallProcessor) buildRequest(ctx context.Context, call definition.CompiledCall, n NodeContext) (fetch.FetchRequest, error) {
	attempts := 0
	result := fetch.FetchRequest{BeforeAttempt: p.budget.attempt, OnAttempt: func() {
		p.progress.attempts.Add(1)
		attempts++
		if attempts > 1 {
			p.progress.retries.Add(1)
		}
	}, OwnerID: p.ownerID, EndpointTemplate: call.Request.Path.Source, Policy: p.engine.FetchPolicy}
	path, err := call.Request.Path.Resolve(n.Values, true)
	if err != nil {
		return result, err
	}
	query := make(map[string]string, len(call.Request.Query))
	names := make([]string, 0, len(n.Values))
	for name := range n.Values {
		if name != "parent" {
			names = append(names, name)
		}
	}
	for key, value := range call.Request.Query {
		if value == nil {
			continue
		}
		if text, ok := value.(string); ok {
			template, err := definition.CompileTemplate(text, names)
			if err != nil {
				return result, err
			}
			resolved, err := template.Resolve(n.Values, false)
			if err != nil {
				return result, err
			}
			query[key] = resolved
			continue
		}
		text, err := definition.ScalarString(value)
		if err != nil {
			return result, err
		}
		query[key] = text
	}
	if call.Pagination != nil && n.Page != nil {
		query[call.Pagination.Page] = strconv.FormatInt(*n.Page, 10)
	}
	resolved, err := fetchapp.BuildRequestURL(p.definition.API.BaseURL, path, query)
	if err != nil {
		return result, &definition.ValidationError{Code: "EGRESS_DENIED", Message: "La ruta no es válida."}
	}
	headers := fetchapp.MergeHeaders(p.definition.API.Headers, call.Request.Headers)
	auth := p.definition.API.Auth
	switch auth.Type {
	case "bearer":
		headers["Authorization"] = []string{"Bearer " + auth.Token}
	case "basic":
		headers["Authorization"] = []string{"Basic " + base64.StdEncoding.EncodeToString([]byte(auth.Username+":"+auth.Password))}
	case "header":
		headers[http.CanonicalHeaderKey(auth.Name)] = []string{auth.Value}
	}
	identity, _ := json.Marshal(auth)
	// Explicit auth defines identity; auth:none intentionally shares provider quota.
	result.CredentialIdentity = string(identity)
	resolver := p.engine.QuotaScopes
	if resolver == nil {
		resolver = fetchapp.NewQuotaScopeResolver(p.engine.IdentityKey)
	}
	provider, err := resolver.ProviderScope(p.definition.API.BaseURL, result.CredentialIdentity)
	if err != nil {
		return result, err
	}
	result.Request = fetch.OutboundRequest{Method: call.Request.Method, URL: resolved, Headers: headers}
	result.Admission = fetch.AdmissionRequest{ExecutionID: p.executionID, ProviderScope: provider}
	result.Admission.Deadline, _ = ctx.Deadline()
	limits := p.definition.API.Limits
	result.Admission.Rules = append(result.Admission.Rules, fetch.QuotaRule{Scope: provider, Requests: int(limits.Requests), Period: limits.Per, Concurrency: limits.Concurrency})
	base, _ := url.Parse(p.definition.API.BaseURL)
	result.EndpointTemplate = strings.TrimSuffix(base.EscapedPath(), "/") + call.Request.Path.Source
	endpoint, err := resolver.EndpointScopeForBase(p.definition.API.BaseURL, provider, call.Request.Method, call.Request.Path.Source)
	if err != nil {
		return result, err
	}
	result.Admission.Rules = append(result.Admission.Rules, fetch.QuotaRule{Scope: endpoint})
	if call.Limits != nil {
		limits = *call.Limits
		result.Admission.Rules[len(result.Admission.Rules)-1] = fetch.QuotaRule{Scope: endpoint, Requests: int(limits.Requests), Period: limits.Per, Concurrency: limits.Concurrency}
	}

	result.Policy.CacheEnabled = call.Cache.Enabled
	result.Policy.CacheTTL = call.Cache.TTL
	return result, nil
}
