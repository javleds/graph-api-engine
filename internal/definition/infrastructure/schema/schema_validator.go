package schema

import (
	"bytes"
	"encoding/json"

	"github.com/javleds/api-engine/internal/api/http/dto"
	"github.com/javleds/api-engine/internal/definition/domain"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

type Validator struct {
	schema     *jsonschema.Schema
	apiVersion int
}

func NewValidator() (*Validator, error) {
	return newVersionValidator(1)
}

func newVersionValidator(version int) (*Validator, error) {
	source, resource := inputSchema, "urn:api-engine:contract:create-execution:v1"
	if version == 2 {
		source, resource = inputSchemaV2, "urn:api-engine:contract:create-execution:v2"
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(source))
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(resource, document); err != nil {
		return nil, err
	}
	compiled, err := compiler.Compile(resource)
	if err != nil {
		return nil, err
	}
	return &Validator{schema: compiled, apiVersion: version}, nil
}
func (v *Validator) Parse(data []byte, maxDepth int) (dto.CreateExecutionRequest, error) {
	var result dto.CreateExecutionRequest
	value, err := DecodeJSON(data, maxDepth)
	if err != nil {
		return result, &domain.ValidationError{Code: "INVALID_JSON", Message: "invalid JSON document"}
	}
	if object, ok := value.(map[string]any); ok {
		if webhook, ok := object["webhook"].(map[string]any); ok && v.apiVersion == 2 && webhook["payload"] == "result" {
			return result, domain.Invalid("/webhook/payload", "V2 webhooks accept only status payloads.")
		}
		if definition, ok := object["definition"].(map[string]any); ok {
			if version, ok := definition["version"].(json.Number); ok && version.String() != "1" {
				return result, &domain.ValidationError{Code: "UNSUPPORTED_VERSION", Path: "/definition/version", Message: "version must be 1"}
			}
		}
	}
	if err := v.schema.Validate(value); err != nil {
		return result, domain.Invalid("", "input does not match the execution contract")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, domain.Invalid("", "input value exceeds supported range")
	}
	// Defaults belong to new V2 requests, never to legacy persisted snapshots.
	if v.apiVersion == 2 && result.Webhook != nil {
		if result.Webhook.Ordering == "" {
			result.Webhook.Ordering = "completion"
		}
		if result.Webhook.MaxInFlight == 0 {
			result.Webhook.MaxInFlight = 1
		}
	}
	return result, nil
}
func Parse(data []byte, maxDepth int) (dto.CreateExecutionRequest, error) {
	return ParseVersion(data, maxDepth, 1)
}

func ParseVersion(data []byte, maxDepth, version int) (dto.CreateExecutionRequest, error) {
	validator, err := newVersionValidator(version)
	if err != nil {
		return dto.CreateExecutionRequest{}, err
	}
	return validator.Parse(data, maxDepth)
}
