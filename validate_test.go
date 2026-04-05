package augur

import (
	"testing"
)

func TestExtractAndParseEnvelope_Direct(t *testing.T) {
	input := `{"data":{"name":"Tom"},"meta":{},"notes":""}`
	env, err := extractAndParseEnvelope(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Data["name"] != "Tom" {
		t.Errorf("unexpected data: %v", env.Data)
	}
}

func TestExtractAndParseEnvelope_MarkdownFence(t *testing.T) {
	input := "Here is the result:\n```json\n{\"data\":{\"x\":1},\"meta\":{},\"notes\":\"\"}\n```"
	env, err := extractAndParseEnvelope(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Data["x"] != float64(1) {
		t.Errorf("unexpected data: %v", env.Data)
	}
}

func TestExtractAndParseEnvelope_BraceScan(t *testing.T) {
	input := `Here is the JSON: {"data":{"y":2},"meta":{},"notes":""} and that's it.`
	env, err := extractAndParseEnvelope(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Data["y"] != float64(2) {
		t.Errorf("unexpected data: %v", env.Data)
	}
}

func TestExtractAndParseEnvelope_Failure(t *testing.T) {
	_, err := extractAndParseEnvelope("this is not json at all")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProcessResponse_FullSuccess(t *testing.T) {
	schema, _ := SchemaFromJSON(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age":  {"type": "integer"}
		},
		"required": ["name", "age"]
	}`)

	raw := `{"data":{"name":"Tom Hanks","age":67},"meta":{"name":{"confidence":0.99,"sources":[]},"age":{"confidence":0.9,"sources":[]}},"notes":""}`

	result, err := processResponse(raw, schema, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.errors) != 0 {
		t.Errorf("expected no errors, got %v", result.errors)
	}
	if result.data["name"] != "Tom Hanks" {
		t.Errorf("name: got %v", result.data["name"])
	}
}

func TestProcessResponse_CoercesStringToInt(t *testing.T) {
	schema, _ := SchemaFromJSON(`{
		"type": "object",
		"properties": {
			"net_worth": {"type": "integer"}
		},
		"required": ["net_worth"]
	}`)

	raw := `{"data":{"net_worth":"$400,000,000"},"meta":{},"notes":""}`

	result, err := processResponse(raw, schema, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.errors) != 0 {
		t.Errorf("expected no errors after coercion, got %v", result.errors)
	}
	if result.data["net_worth"] != float64(400000000) {
		t.Errorf("net_worth: got %v, want 400000000", result.data["net_worth"])
	}
}

func TestProcessResponse_AppliesDefault(t *testing.T) {
	schema, _ := SchemaFromType[withDefault]()

	raw := `{"data":{"amount":100},"meta":{},"notes":""}`

	result, err := processResponse(raw, schema, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.data["currency"] != "USD" {
		t.Errorf("currency default not applied, got %v", result.data["currency"])
	}
}

func TestProcessResponse_MissingRequired(t *testing.T) {
	schema, _ := SchemaFromJSON(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age":  {"type": "integer"}
		},
		"required": ["name", "age"]
	}`)

	raw := `{"data":{"name":"Tom Hanks"},"meta":{},"notes":""}`

	result, err := processResponse(raw, schema, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	failed := requiredFailures(result, schema)
	if len(failed) != 1 || failed[0].Field != "age" {
		t.Errorf("expected age to be a required failure, got %v", failed)
	}
}

func TestProcessResponse_MalformedJSON(t *testing.T) {
	schema, _ := SchemaFromJSON(`{"type":"object","properties":{"x":{"type":"string"}}}`)

	_, err := processResponse("not json", schema, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMergeRetryResult(t *testing.T) {
	base := &processResult{
		data: map[string]any{"name": "Tom"},
		errors: []FieldError{
			{Field: "age", Reason: "missing"},
		},
		meta: map[string]map[string]any{},
	}
	retry := &processResult{
		data:   map[string]any{"age": float64(67)},
		errors: []FieldError{},
		meta:   map[string]map[string]any{"age": {"confidence": float64(0.9)}},
	}

	mergeRetryResult(base, retry)

	if base.data["name"] != "Tom" {
		t.Error("original field should be preserved")
	}
	if base.data["age"] != float64(67) {
		t.Errorf("retry field not merged, got %v", base.data["age"])
	}
	if len(base.errors) != 0 {
		t.Errorf("resolved error should be removed, got %v", base.errors)
	}
}
