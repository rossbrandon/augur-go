package augur

import (
	"fmt"
	"reflect"
	"strings"
)

// augurTagDirectives holds parsed directives from an `augur:"..."` struct tag.
type augurTagDirectives struct {
	required   bool
	desc       string
	defaultVal any
}

// reflectSchema derives a JSON Schema map and fieldSchema index from a Go value
// using reflection and augur struct tags.
func reflectSchema(v any) (map[string]any, map[string]*fieldSchema, error) {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil, nil, fmt.Errorf("cannot reflect nil value")
	}

	// Dereference pointer.
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil, nil, fmt.Errorf("SchemaFromType requires a struct type, got %s", t.Kind())
	}

	return reflectStruct(t)
}

// reflectStruct converts a struct type into a JSON Schema map and fieldSchema index.
func reflectStruct(t reflect.Type) (map[string]any, map[string]*fieldSchema, error) {
	properties := make(map[string]any)
	fields := make(map[string]*fieldSchema)
	var required []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields.
		if !field.IsExported() {
			continue
		}

		// Get JSON field name.
		jsonName := jsonFieldName(field)
		if jsonName == "-" {
			continue
		}

		// Parse augur struct tag.
		augurTag := parseAugurTag(field.Tag.Get("augur"))

		// Build the JSON Schema entry for this field.
		fieldType := field.Type
		// Dereference pointer — pointer types are nullable.
		for fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}

		propSchema, fs, err := reflectFieldType(jsonName, fieldType)
		if err != nil {
			return nil, nil, fmt.Errorf("field %q: %w", jsonName, err)
		}

		fs.Required = augurTag.required
		fs.Description = augurTag.desc
		fs.Default = augurTag.defaultVal

		if augurTag.desc != "" {
			propSchema["description"] = augurTag.desc
		}
		if augurTag.defaultVal != nil {
			propSchema["default"] = augurTag.defaultVal
		}
		if augurTag.required {
			required = append(required, jsonName)
		}

		properties[jsonName] = propSchema
		fields[jsonName] = fs
	}

	raw := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		raw["required"] = required
	}

	return raw, fields, nil
}

// reflectFieldType converts a reflect.Type into a JSON Schema property map and
// a fieldSchema skeleton (Name and Type only; caller fills Required/Description/Default).
func reflectFieldType(name string, t reflect.Type) (map[string]any, *fieldSchema, error) {
	fs := &fieldSchema{Name: name}
	prop := make(map[string]any)

	switch t.Kind() {
	case reflect.String:
		prop["type"] = "string"
		fs.Type = "string"

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		prop["type"] = "integer"
		fs.Type = "integer"

	case reflect.Float32, reflect.Float64:
		prop["type"] = "number"
		fs.Type = "number"

	case reflect.Bool:
		prop["type"] = "boolean"
		fs.Type = "boolean"

	case reflect.Slice:
		prop["type"] = "array"
		fs.Type = "array"

		elemType := t.Elem()
		for elemType.Kind() == reflect.Pointer {
			elemType = elemType.Elem()
		}

		itemsProp, itemsFS, err := reflectFieldType("", elemType)
		if err != nil {
			return nil, nil, fmt.Errorf("array items: %w", err)
		}
		prop["items"] = itemsProp
		fs.Items = itemsFS

	case reflect.Struct:
		prop["type"] = "object"
		fs.Type = "object"

		nestedRaw, nestedFields, err := reflectStruct(t)
		if err != nil {
			return nil, nil, err
		}
		if nestedProps, ok := nestedRaw["properties"]; ok {
			prop["properties"] = nestedProps
		}
		if nestedReq, ok := nestedRaw["required"]; ok {
			prop["required"] = nestedReq
		}
		fs.Properties = nestedFields

	case reflect.Map:
		prop["type"] = "object"
		fs.Type = "object"
		// For map[string]T, set additionalProperties to the value schema.
		if t.Key().Kind() == reflect.String {
			valType := t.Elem()
			for valType.Kind() == reflect.Pointer {
				valType = valType.Elem()
			}
			valProp, _, err := reflectFieldType("", valType)
			if err != nil {
				return nil, nil, fmt.Errorf("map value: %w", err)
			}
			prop["additionalProperties"] = valProp
		}

	default:
		return nil, nil, fmt.Errorf("unsupported field type: %s", t.Kind())
	}

	return prop, fs, nil
}

// parseAugurTag parses the value of an `augur` struct tag.
// Format: `augur:"required,desc:some text,default:value"`
// Commas inside desc: or default: values are preserved (splits only at directive boundaries).
func parseAugurTag(tag string) augurTagDirectives {
	if tag == "" {
		return augurTagDirectives{}
	}

	var d augurTagDirectives
	for _, part := range splitAugurDirectives(tag) {
		part = strings.TrimSpace(part)
		switch {
		case part == "required":
			d.required = true
		case strings.HasPrefix(part, "desc:"):
			d.desc = strings.TrimPrefix(part, "desc:")
		case strings.HasPrefix(part, "default:"):
			d.defaultVal = strings.TrimPrefix(part, "default:")
		}
	}

	return d
}

// splitAugurDirectives splits a tag string on commas that immediately precede a
// known directive keyword, preserving commas inside desc: and default: values.
func splitAugurDirectives(tag string) []string {
	knownDirectives := []string{"required", "desc:", "default:"}
	var parts []string
	start := 0
	for i := 0; i < len(tag); i++ {
		if tag[i] != ',' {
			continue
		}
		rest := strings.TrimSpace(tag[i+1:])
		for _, kw := range knownDirectives {
			if strings.HasPrefix(rest, kw) {
				parts = append(parts, tag[start:i])
				start = i + 1
				break
			}
		}
	}
	parts = append(parts, tag[start:])
	return parts
}

// jsonFieldName returns the JSON field name for a struct field, using the
// `json` tag if present, or the field name otherwise.
func jsonFieldName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" {
		return f.Name
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		return f.Name
	}
	return name
}
