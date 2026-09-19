package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var environmentReference = regexp.MustCompile("\\$\\{([A-Za-z_][A-Za-z0-9_]*)\\}")

func decodeYAML(data []byte, lookup func(string) (string, bool)) (*Config, error) {
	document, err := readYAMLDocument(data)
	if err != nil {
		return nil, err
	}
	if err := validateYAMLNode(document.Content[0]); err != nil {
		return nil, err
	}
	if err := interpolateYAMLNode(document.Content[0], lookup); err != nil {
		return nil, err
	}
	if err := validateYAMLNode(document.Content[0]); err != nil {
		return nil, err
	}
	var value any
	if err := document.Content[0].Decode(&value); err != nil {
		return nil, fmt.Errorf("invalid configuration YAML: %w", err)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("normalize configuration YAML: %w", err)
	}
	return decodeJSON(canonical)
}

func readYAMLDocument(data []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("configuration YAML must contain one document")
		}
		return nil, fmt.Errorf("invalid configuration YAML: %w", err)
	}
	if len(document.Content) != 1 {
		return nil, fmt.Errorf("configuration YAML must contain one document")
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("configuration YAML must contain one document")
		}
		return nil, fmt.Errorf("invalid configuration YAML: %w", err)
	}
	return &document, nil
}

func validateYAMLNode(node *yaml.Node) error {
	switch node.Kind {
	case yaml.AliasNode:
		return fmt.Errorf("configuration YAML aliases are not allowed")
	case yaml.MappingNode:
		keys := make(map[string]struct{}, len(node.Content)/2)
		for index := 0; index < len(node.Content); index += 2 {
			key, value := node.Content[index], node.Content[index+1]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
				return fmt.Errorf("configuration YAML keys must be strings")
			}
			if key.Value == "<<" {
				return fmt.Errorf("configuration YAML merge keys are not allowed")
			}
			if _, exists := keys[key.Value]; exists {
				return fmt.Errorf("configuration YAML contains duplicate key %q", key.Value)
			}
			keys[key.Value] = struct{}{}
			if err := validateYAMLNode(value); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			if err := validateYAMLNode(child); err != nil {
				return err
			}
		}
	case yaml.ScalarNode:
		if node.Tag == "!!null" {
			return fmt.Errorf("configuration values cannot be null")
		}
	}
	return nil
}

func interpolateYAMLNode(node *yaml.Node, lookup func(string) (string, bool)) error {
	switch node.Kind {
	case yaml.MappingNode:
		for index := 1; index < len(node.Content); index += 2 {
			if err := interpolateYAMLNode(node.Content[index], lookup); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			if err := interpolateYAMLNode(child, lookup); err != nil {
				return err
			}
		}
	case yaml.ScalarNode:
		return interpolateYAMLScalar(node, lookup)
	}
	return nil
}

func interpolateYAMLScalar(node *yaml.Node, lookup func(string) (string, bool)) error {
	matches := environmentReference.FindAllStringSubmatchIndex(node.Value, -1)
	if len(matches) == 0 {
		return nil
	}
	var value strings.Builder
	position := 0
	for _, match := range matches {
		value.WriteString(node.Value[position:match[0]])
		name := node.Value[match[2]:match[3]]
		resolved, found := lookup(name)
		if !found || resolved == "" {
			return fmt.Errorf("environment variable %s is required", name)
		}
		value.WriteString(resolved)
		position = match[1]
	}
	value.WriteString(node.Value[position:])
	result := value.String()
	if matches[0][0] != 0 || matches[len(matches)-1][1] != len(node.Value) || len(matches) != 1 {
		node.Value = result
		node.Tag = "!!str"
		return nil
	}
	if strings.ContainsAny(result, "\r\n") {
		return fmt.Errorf("environment variable %s must resolve to one YAML scalar", node.Value[matches[0][2]:matches[0][3]])
	}
	var scalar yaml.Node
	if err := yaml.Unmarshal([]byte(result), &scalar); err != nil || len(scalar.Content) != 1 || scalar.Content[0].Kind != yaml.ScalarNode {
		return fmt.Errorf("environment variable %s must resolve to one YAML scalar", node.Value[matches[0][2]:matches[0][3]])
	}
	node.Value = scalar.Content[0].Value
	node.Tag = scalar.Content[0].Tag
	node.Style = scalar.Content[0].Style
	return nil
}
