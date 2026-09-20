package schema

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/javleds/api-engine/internal/api/http/mapper"
	"github.com/javleds/api-engine/internal/definition/application"
	"github.com/javleds/api-engine/internal/definition/domain"
)

func TestV2WebhookPolicyDefaultsAndCompilation(t *testing.T) {
	for _, test := range []webhookPolicyCase{
		{"defaults", "", "completion", 1},
		{"source", `,"ordering":"source"`, "source", 1},
		{"concurrent", `,"maxInFlight":4`, "completion", 4},
		{"source concurrent", `,"ordering":"source","maxInFlight":4`, "source", 4},
		{"unlimited", `,"maxInFlight":0`, "completion", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := validInput[:len(validInput)-1] + `,"webhook":{"url":"https://receiver.example.com"` + test.fields + `}}`
			parsed, err := ParseVersion([]byte(input), 64, 2)
			if err != nil {
				t.Fatal(err)
			}
			compiled, err := mapper.Compile(parsed, application.DefaultValidationLimits())
			if err != nil {
				t.Fatal(err)
			}
			if compiled.Webhook.Ordering != test.ordering || compiled.Webhook.MaxInFlight != test.inFlight {
				t.Fatalf("unexpected policy: %+v", compiled.Webhook)
			}
			if test.fields != "" {
				if _, err := ParseVersion([]byte(input), 64, 1); err == nil {
					t.Fatal("V1 accepted V2 policy")
				}
			}
		})
	}
	var legacy domain.Webhook
	if err := json.Unmarshal([]byte(`{"URL":"https://receiver.example.com"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.Ordering != "" || legacy.MaxInFlight != 0 {
		t.Fatal("legacy policy changed")
	}
	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("Ordering")) || bytes.Contains(encoded, []byte("MaxInFlight")) {
		t.Fatal("legacy snapshot acquired new fields, changing its fingerprint")
	}
}

func TestV2WebhookPolicyRejectsInvalidValues(t *testing.T) {
	for _, field := range []string{
		`"ordering":""`, `"ordering":"random"`, `"ordering":null`, `"ordering":1`,
		`"maxInFlight":-1`, `"maxInFlight":1.5`, `"maxInFlight":"2"`,
		`"maxInFlight":null`, `"maxInFlight":99999999999999999999999999999999`,
	} {
		input := validInput[:len(validInput)-1] + `,"webhook":{"url":"https://receiver.example.com",` + field + `}}`
		if _, err := ParseVersion([]byte(input), 64, 2); err == nil {
			t.Errorf("accepted %s", field)
		}
	}
}

func TestV2EmbeddedSchemaEqualsSource(t *testing.T) {
	source, err := os.ReadFile("../../../../contracts/create_execution_v2.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(source, inputSchemaV2) {
		t.Fatal("V2 contract differs from runtime schema")
	}
}
