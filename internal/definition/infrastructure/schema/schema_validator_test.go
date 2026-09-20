package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/javleds/api-engine/internal/definition/domain"
)

const validInput = `{"definition":{"version":1,"api":{"baseUrl":"https://example.com/api","limits":{"requests":100,"per":"1m"}},"calls":[{"name":"items","type":"collection","request":{"method":"GET","path":"/items","query":{"id":9007199254740993}}}]}}`

func TestStrictJSON(t *testing.T) {
	for _, input := range []string{`{"a":1,"a":2}`, `{"a":{"b":1,"b":2}}`, `{} {}`, `[1,]`, `{"a":`} {
		if _, err := DecodeJSON([]byte(input), 64); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
	if _, err := DecodeJSON([]byte(`[[[1]]]`), 2); err == nil {
		t.Fatal("depth limit ignored")
	}
}
func TestSchemaContract(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := validator.Parse([]byte(validInput), 64)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Definition.Calls.Calls[0].Request.Query["id"] != json.Number("9007199254740993") {
		t.Fatal("number lost precision")
	}
	if len(parsed.Definition.Calls.Calls) != 1 {
		t.Fatal("root array was not decoded")
	}
	if _, err := validator.Parse([]byte(strings.Replace(validInput, `"calls":[{`, `"calls":{`, 1)), 64); err == nil {
		t.Fatal("accepted object calls")
	}
	for _, input := range []string{
		strings.Replace(validInput, `"name":"items"`, `"name":"items","foreach":true`, 1),
		strings.Replace(validInput, `"id":9007199254740993`, `"id":null`, 1),
		strings.Replace(validInput, `"requests":100`, `"requests":100,"requests":200`, 1),
		strings.Replace(validInput, `"method":"GET"`, `"method":"POST"`, 1),
	} {
		if _, err := validator.Parse([]byte(input), 64); err == nil {
			t.Errorf("accepted invalid input %s", input)
		}
	}
	_, err = validator.Parse([]byte(strings.Replace(validInput, `"version":1`, `"version":2`, 1)), 64)
	var failure *domain.ValidationError
	if !errors.As(err, &failure) || failure.Code != "UNSUPPORTED_VERSION" {
		t.Fatal(err)
	}
}
func TestEmbeddedSchemaEqualsSource(t *testing.T) {
	for _, path := range []string{"../../../../contracts/create_execution_v1.schema.json", "../../../../api_request_schema.json"} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(source, inputSchema) {
			t.Fatalf("%s differs from embedded runtime schema", path)
		}
	}
}
func FuzzDecodeJSON(f *testing.F) {
	f.Add(validInput)
	f.Add(`{"a":1,"a":2}`)
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 262144 {
			t.Skip()
		}
		_, _ = DecodeJSON([]byte(input), 64)
	})
}
