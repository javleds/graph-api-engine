package domain

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestDurationBounds(t *testing.T) {
	value, err := ParseDuration("7d")
	if err != nil || value != 7*24*time.Hour {
		t.Fatalf("%v %v", value, err)
	}
	for _, invalid := range []string{"0s", "-1s", "1.5s", "1y", "01s", "9223372036854775807d", "1h30m"} {
		if _, err := ParseDuration(invalid); err == nil {
			t.Errorf("accepted %s", invalid)
		}
	}
}
func TestSelectorDistinguishesMissingNullAndNumericObjectKey(t *testing.T) {
	value := map[string]any{"data": []any{nil, map[string]any{"attr": json.Number("9007199254740993")}}, "1": nil}
	selected, err := Select(value, []string{"data", "1", "attr"})
	if err != nil || selected != json.Number("9007199254740993") {
		t.Fatalf("%v %v", selected, err)
	}
	selected, err = Select(value, []string{"1"})
	if err != nil || selected != nil {
		t.Fatalf("existing null: %v", err)
	}
	_, err = Select(value, []string{"missing"})
	var failure *ValidationError
	if !errors.As(err, &failure) || failure.Code != "SELECT_PATH_NOT_FOUND" {
		t.Fatal(err)
	}
	for _, invalid := range []string{"a..b", "a[]", "$", "a.*", "a()"} {
		if _, err := ParseDotPath(invalid); err == nil {
			t.Errorf("accepted %q", invalid)
		}
	}
}
func TestTemplateBindingPreservesPrecisionAndEscapes(t *testing.T) {
	template, err := CompileTemplate("/items/{parent.id}/{parent.name}", []string{"items"})
	if err != nil {
		t.Fatal(err)
	}
	value, err := template.Resolve(map[string]any{"parent": map[string]any{"id": json.Number("9007199254740993"), "name": "a/b?x#y"}}, true)
	if err != nil || value != "/items/9007199254740993/a%2Fb%3Fx%23y" {
		t.Fatalf("%q %v", value, err)
	}
	for _, invalid := range []string{"{missing.id}", "{parent}", "{parent.id", "parent.id}", "{{parent.id}"} {
		if _, err := CompileTemplate(invalid, []string{"items"}); err == nil {
			t.Errorf("accepted %q", invalid)
		}
	}
	if _, err := CompileTemplate("{items.id}", []string{"items", "items"}); err == nil {
		t.Fatal("ambiguous ancestor accepted")
	}
	if _, err := CompileTemplate("{parent.id}", nil); err == nil {
		t.Fatal("root reference accepted")
	}
	template, _ = CompileTemplate("/{parent.id}", []string{"items"})
	for _, invalid := range []any{nil, []any{}, map[string]any{}, "..", "a\nb"} {
		if _, err := template.Resolve(map[string]any{"parent": map[string]any{"id": invalid}}, true); err == nil {
			t.Errorf("accepted %#v", invalid)
		}
	}
}
