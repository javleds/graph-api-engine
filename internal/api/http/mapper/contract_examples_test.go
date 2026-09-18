package mapper

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/javleds/api-engine/internal/definition/application"
	"github.com/javleds/api-engine/internal/definition/infrastructure/schema"
)

func TestNormativeInputExamplesCompile(t *testing.T) {
	for _, name := range []string{"pokemon_full.json", "input_02.json"} {
		data, err := os.ReadFile("../../../../contracts/examples/" + name)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := schema.Parse(data, 64)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Compile(parsed, application.DefaultValidationLimits()); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
func TestNullAndOmittedSelectHaveIdenticalCompilation(t *testing.T) {
	plain, err := schema.Parse([]byte(input), 64)
	if err != nil {
		t.Fatal(err)
	}
	withNull, err := schema.Parse([]byte(strings.Replace(input, `"name":"items"`, `"name":"items","select":null`, 1)), 64)
	if err != nil {
		t.Fatal(err)
	}
	left, err := Compile(plain, application.DefaultValidationLimits())
	if err != nil {
		t.Fatal(err)
	}
	right, err := Compile(withNull, application.DefaultValidationLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(left, right) {
		t.Fatal("null select differs from omission")
	}
}
