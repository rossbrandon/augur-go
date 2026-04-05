package augur

import (
	"encoding/json"
	"fmt"
	"os"
)

// Schema is the internal representation of a JSON Schema.
// All schema inputs (struct reflection, JSON string, file) are normalized
// into this type before use.
type Schema struct {
	raw        map[string]any
	required   []string
	properties map[string]*fieldSchema
}

// fieldSchema describes a single field in the schema. It is an internal type
// used by the validation and coercion pipeline; it is not part of the public API.
type fieldSchema struct {
	Name        string
	Type        string // "string", "integer", "number", "boolean", "array", "object"
	Description string
	Required    bool
	Default     any
	Items       *fieldSchema            // for array types
	Properties  map[string]*fieldSchema // for object types
}

// SchemaFromType derives a JSON Schema from a Go struct type T using reflection
// and augur struct tags. This is the primary schema input path for SDK users.
func SchemaFromType[T any]() (*Schema, error) {
	var zero T
	raw, fields, err := reflectSchema(zero)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSchemaInvalid, err)
	}
	return buildSchema(raw, fields)
}

// SchemaFromJSON parses a JSON Schema from a raw JSON string.
func SchemaFromJSON(jsonStr string) (*Schema, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("%w: invalid JSON: %s", ErrSchemaInvalid, err)
	}
	return parseRawSchema(raw)
}

// SchemaFromFile reads and parses a JSON Schema from a file path.
func SchemaFromFile(path string) (*Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot read file %q: %s", ErrSchemaInvalid, path, err)
	}
	return SchemaFromJSON(string(data))
}

// ToJSON serializes the schema to a JSON string for embedding in prompts.
func (s *Schema) ToJSON() (string, error) {
	b, err := json.MarshalIndent(s.raw, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// RequiredFields returns the list of required top-level field names.
func (s *Schema) RequiredFields() []string {
	return s.required
}

// buildSchema constructs a Schema from a raw JSON Schema map and a parsed
// properties map derived from reflection.
func buildSchema(raw map[string]any, fields map[string]*fieldSchema) (*Schema, error) {
	s := &Schema{
		raw:        raw,
		properties: fields,
	}

	// Extract required fields from raw schema.
	if req, ok := raw["required"]; ok {
		switch v := req.(type) {
		case []any:
			for _, r := range v {
				if name, ok := r.(string); ok {
					s.required = append(s.required, name)
				}
			}
		case []string:
			s.required = v
		}
	}

	return s, nil
}

// parseRawSchema constructs a Schema from an externally-provided JSON Schema map.
func parseRawSchema(raw map[string]any) (*Schema, error) {
	props, err := parseProperties(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSchemaInvalid, err)
	}
	return buildSchema(raw, props)
}

// parseProperties extracts fieldSchema entries from a raw JSON Schema map.
func parseProperties(raw map[string]any) (map[string]*fieldSchema, error) {
	fields := make(map[string]*fieldSchema)

	propsRaw, ok := raw["properties"]
	if !ok {
		return fields, nil
	}

	propsMap, ok := propsRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("properties must be an object")
	}

	// Determine required set.
	requiredSet := make(map[string]bool)
	if req, ok := raw["required"]; ok {
		if arr, ok := req.([]any); ok {
			for _, r := range arr {
				if name, ok := r.(string); ok {
					requiredSet[name] = true
				}
			}
		}
	}

	for name, fieldRaw := range propsMap {
		fieldMap, ok := fieldRaw.(map[string]any)
		if !ok {
			continue
		}
		fs, err := parseSingleField(name, fieldMap, requiredSet[name])
		if err != nil {
			return nil, err
		}
		fields[name] = fs
	}

	return fields, nil
}

// parseSingleField constructs a fieldSchema from a raw field map.
func parseSingleField(name string, fieldMap map[string]any, required bool) (*fieldSchema, error) {
	fs := &fieldSchema{
		Name:     name,
		Required: required,
	}
	if t, ok := fieldMap["type"].(string); ok {
		fs.Type = t
	}
	if d, ok := fieldMap["description"].(string); ok {
		fs.Description = d
	}
	if def, ok := fieldMap["default"]; ok {
		fs.Default = def
	}
	if fs.Type == "array" {
		if itemsRaw, ok := fieldMap["items"].(map[string]any); ok {
			items, err := parseSingleField("", itemsRaw, false)
			if err != nil {
				return nil, err
			}
			fs.Items = items
		}
	}
	if fs.Type == "object" {
		nested, err := parseProperties(fieldMap)
		if err != nil {
			return nil, err
		}
		fs.Properties = nested
	}
	return fs, nil
}
