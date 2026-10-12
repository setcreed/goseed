package generator

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

var resourceFieldName = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)

// ResourceSpec describes the deliberately small, versioned resource field DSL.
type ResourceSpec struct {
	Version int
	Fields  []ResourceField
}

// ResourceField is a client-writable field in a generated resource.
type ResourceField struct {
	Name      string
	GoName    string
	Type      string
	Required  bool
	MaxLength int
}

// ParseResourceSpec parses one strict version 1 resource document.
func ParseResourceSpec(data []byte) (ResourceSpec, error) {
	if !utf8.Valid(data) {
		return ResourceSpec{}, fmt.Errorf("resource spec must be valid UTF-8")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return ResourceSpec{}, fmt.Errorf("parse resource spec: %w", err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return ResourceSpec{}, fmt.Errorf("resource spec must contain one mapping document")
	}
	if err := validateResourceNode(document.Content[0]); err != nil {
		return ResourceSpec{}, err
	}
	if err := validateResourceShape(document.Content[0]); err != nil {
		return ResourceSpec{}, err
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return ResourceSpec{}, fmt.Errorf("resource spec must contain one document")
		}
		return ResourceSpec{}, fmt.Errorf("parse resource spec: %w", err)
	}

	var raw struct {
		Version *int `yaml:"version"`
		Fields  []struct {
			Name      *string `yaml:"name"`
			Type      *string `yaml:"type"`
			Required  *bool   `yaml:"required"`
			MaxLength *int    `yaml:"max_length"`
		} `yaml:"fields"`
	}
	strict := yaml.NewDecoder(bytes.NewReader(data))
	strict.KnownFields(true)
	if err := strict.Decode(&raw); err != nil {
		return ResourceSpec{}, fmt.Errorf("decode resource spec: %w", err)
	}
	if raw.Version == nil || *raw.Version != 1 {
		return ResourceSpec{}, fmt.Errorf("resource spec version must be 1")
	}
	if len(raw.Fields) < 1 || len(raw.Fields) > 32 {
		return ResourceSpec{}, fmt.Errorf("resource spec must declare 1 to 32 fields")
	}
	result := ResourceSpec{Version: 1, Fields: make([]ResourceField, 0, len(raw.Fields))}
	seen := make(map[string]bool, len(raw.Fields))
	goNames := make(map[string]bool, len(raw.Fields))
	for i, field := range raw.Fields {
		if field.Name == nil || !resourceFieldName.MatchString(*field.Name) || len(*field.Name) > 63 {
			return ResourceSpec{}, fmt.Errorf("field %d name must be lower snake_case with at most 63 ASCII characters", i+1)
		}
		if isReservedResourceField(*field.Name) {
			return ResourceSpec{}, fmt.Errorf("field %q is reserved", *field.Name)
		}
		if seen[*field.Name] {
			return ResourceSpec{}, fmt.Errorf("duplicate resource field %q", *field.Name)
		}
		seen[*field.Name] = true
		if field.Type == nil || (*field.Type != "string" && *field.Type != "bool" && *field.Type != "int64") {
			return ResourceSpec{}, fmt.Errorf("field %q type must be string, bool, or int64", *field.Name)
		}
		maxLength := 0
		if *field.Type == "string" {
			maxLength = 255
		}
		if field.MaxLength != nil {
			if *field.Type != "string" {
				return ResourceSpec{}, fmt.Errorf("field %q max_length is only valid for string", *field.Name)
			}
			if *field.MaxLength < 1 || *field.MaxLength > 4096 {
				return ResourceSpec{}, fmt.Errorf("field %q max_length must be between 1 and 4096", *field.Name)
			}
			maxLength = *field.MaxLength
		}
		goName := resourceGoName(*field.Name)
		if goNames[goName] {
			return ResourceSpec{}, fmt.Errorf("resource fields produce duplicate Go name %q", goName)
		}
		goNames[goName] = true
		required := field.Required != nil && *field.Required
		result.Fields = append(result.Fields, ResourceField{Name: *field.Name, GoName: goName, Type: *field.Type, Required: required, MaxLength: maxLength})
	}
	return result, nil
}

func validateResourceNode(node *yaml.Node) error {
	if node.Anchor != "" || node.Kind == yaml.AliasNode || node.ShortTag() == "!!merge" {
		return fmt.Errorf("resource spec does not support aliases, anchors, or merge keys")
	}
	if node.Tag != "" && node.Tag != "!" && !strings.HasPrefix(node.Tag, "tag:yaml.org,2002:") && !strings.HasPrefix(node.Tag, "!!") {
		return fmt.Errorf("resource spec does not support custom YAML tags")
	}
	if node.Kind == yaml.ScalarNode && node.ShortTag() == "!!null" {
		return fmt.Errorf("resource spec does not support null values")
	}
	if node.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.ShortTag() != "!!str" {
				return fmt.Errorf("resource spec mapping keys must be strings")
			}
			if seen[key.Value] {
				return fmt.Errorf("duplicate resource spec key %q", key.Value)
			}
			seen[key.Value] = true
		}
	}
	for _, child := range node.Content {
		if err := validateResourceNode(child); err != nil {
			return err
		}
	}
	return nil
}

func validateResourceShape(root *yaml.Node) error {
	rootKeys := resourceMapping(root)
	if rootKeys == nil {
		return fmt.Errorf("resource spec must be a mapping")
	}
	for key, tag := range map[string]string{"version": "!!int", "fields": "!!seq"} {
		value := rootKeys[key]
		if value == nil || (key == "version" && value.ShortTag() != tag) || (key == "fields" && value.Kind != yaml.SequenceNode) {
			return fmt.Errorf("resource spec %s has an invalid or missing value", key)
		}
	}
	fields := rootKeys["fields"]
	for index, field := range fields.Content {
		values := resourceMapping(field)
		if values == nil {
			return fmt.Errorf("resource field %d must be a mapping", index+1)
		}
		for key, tag := range map[string]string{"name": "!!str", "type": "!!str"} {
			value := values[key]
			if value == nil || value.Kind != yaml.ScalarNode || value.ShortTag() != tag {
				return fmt.Errorf("resource field %d %s must be a string", index+1, key)
			}
		}
		if value := values["required"]; value != nil && (value.Kind != yaml.ScalarNode || value.ShortTag() != "!!bool") {
			return fmt.Errorf("resource field %d required must be a boolean", index+1)
		}
		if value := values["max_length"]; value != nil && (value.Kind != yaml.ScalarNode || value.ShortTag() != "!!int") {
			return fmt.Errorf("resource field %d max_length must be an integer", index+1)
		}
	}
	return nil
}

func resourceMapping(node *yaml.Node) map[string]*yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	values := make(map[string]*yaml.Node, len(node.Content)/2)
	for index := 0; index < len(node.Content); index += 2 {
		values[node.Content[index].Value] = node.Content[index+1]
	}
	return values
}

func isReservedResourceField(name string) bool {
	switch name {
	case "id", "owner", "created_at", "updated_at":
		return true
	default:
		return false
	}
}

func resourceGoName(name string) string {
	parts := strings.Split(name, "_")
	var result strings.Builder
	for _, part := range parts {
		result.WriteByte(part[0] - ('a' - 'A'))
		result.WriteString(part[1:])
	}
	return result.String()
}
