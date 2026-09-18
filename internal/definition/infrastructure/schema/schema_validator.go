package schema

import (
	"bytes"
	"encoding/json"

	"github.com/javleds/api-engine/internal/api/http/dto"
	"github.com/javleds/api-engine/internal/definition/domain"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

type Validator struct{ schema *jsonschema.Schema }

func NewValidator() (*Validator, error) {
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(inputSchema))
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("urn:api-engine:contract:create-execution:v1", document); err != nil {
		return nil, err
	}
	compiled, err := compiler.Compile("urn:api-engine:contract:create-execution:v1")
	if err != nil {
		return nil, err
	}
	return &Validator{schema: compiled}, nil
}
func (v *Validator) Parse(data []byte, maxDepth int) (dto.CreateExecutionRequest, error) {
	var result dto.CreateExecutionRequest
	value, err := DecodeJSON(data, maxDepth)
	if err != nil {
		return result, &domain.ValidationError{Code: "INVALID_JSON", Message: "invalid JSON document"}
	}
	if object, ok := value.(map[string]any); ok {
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
	return result, nil
}
func Parse(data []byte, maxDepth int) (dto.CreateExecutionRequest, error) {
	validator, err := NewValidator()
	if err != nil {
		return dto.CreateExecutionRequest{}, err
	}
	return validator.Parse(data, maxDepth)
}
