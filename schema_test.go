package augur

import (
	"testing"
)

type simpleStruct struct {
	Name   string  `json:"name"  augur:"required,desc:Full name"`
	Age    int     `json:"age"   augur:"required,desc:Age in years"`
	Score  float64 `json:"score" augur:"desc:Score between 0 and 1"`
	Active bool    `json:"active"`
}

type nestedStruct struct {
	Title    string       `json:"title"    augur:"required"`
	Tags     []string     `json:"tags"     augur:"desc:List of tags"`
	Metadata simpleStruct `json:"metadata"`
}

type withDefault struct {
	Currency string `json:"currency" augur:"required,default:USD"`
	Amount   int    `json:"amount"   augur:"required"`
}

func TestSchemaFromType_SimpleStruct(t *testing.T) {
	s, err := SchemaFromType[simpleStruct]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props := s.properties
	if len(props) != 4 {
		t.Errorf("expected 4 properties, got %d", len(props))
	}

	nameFS := props["name"]
	if nameFS == nil {
		t.Fatal("missing 'name' property")
	}
	if nameFS.Type != "string" {
		t.Errorf("name type: got %q, want %q", nameFS.Type, "string")
	}
	if !nameFS.Required {
		t.Error("name should be required")
	}
	if nameFS.Description != "Full name" {
		t.Errorf("name description: got %q, want %q", nameFS.Description, "Full name")
	}

	ageFS := props["age"]
	if ageFS == nil || ageFS.Type != "integer" {
		t.Errorf("age type: got %v", ageFS)
	}

	scoreFS := props["score"]
	if scoreFS == nil || scoreFS.Type != "number" {
		t.Errorf("score type: got %v", scoreFS)
	}
	if scoreFS.Required {
		t.Error("score should not be required")
	}

	activeFS := props["active"]
	if activeFS == nil || activeFS.Type != "boolean" {
		t.Errorf("active type: got %v", activeFS)
	}
}

func TestSchemaFromType_RequiredFields(t *testing.T) {
	s, err := SchemaFromType[simpleStruct]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	required := s.RequiredFields()
	requiredSet := make(map[string]bool)
	for _, r := range required {
		requiredSet[r] = true
	}

	if !requiredSet["name"] {
		t.Error("expected 'name' in required fields")
	}
	if !requiredSet["age"] {
		t.Error("expected 'age' in required fields")
	}
	if requiredSet["score"] {
		t.Error("'score' should not be in required fields")
	}
}

func TestSchemaFromType_NestedStruct(t *testing.T) {
	s, err := SchemaFromType[nestedStruct]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props := s.properties

	tagsFS := props["tags"]
	if tagsFS == nil || tagsFS.Type != "array" {
		t.Fatalf("tags type: got %v", tagsFS)
	}
	if tagsFS.Items == nil || tagsFS.Items.Type != "string" {
		t.Errorf("tags items type: got %v", tagsFS.Items)
	}

	metaFS := props["metadata"]
	if metaFS == nil || metaFS.Type != "object" {
		t.Fatalf("metadata type: got %v", metaFS)
	}
	if metaFS.Properties == nil {
		t.Fatal("metadata should have nested properties")
	}
	if metaFS.Properties["name"] == nil {
		t.Error("metadata.name should exist")
	}
}

func TestSchemaFromType_Default(t *testing.T) {
	s, err := SchemaFromType[withDefault]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	currencyFS := s.properties["currency"]
	if currencyFS == nil {
		t.Fatal("missing 'currency' property")
	}
	if currencyFS.Default != "USD" {
		t.Errorf("currency default: got %v, want %q", currencyFS.Default, "USD")
	}
}

func TestSchemaFromJSON(t *testing.T) {
	const jsonSchema = `{
		"type": "object",
		"properties": {
			"net_worth": {"type": "integer", "description": "Estimated net worth in USD"},
			"currency":  {"type": "string",  "default": "USD"}
		},
		"required": ["net_worth"]
	}`

	s, err := SchemaFromJSON(jsonSchema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props := s.properties
	nwFS := props["net_worth"]
	if nwFS == nil || nwFS.Type != "integer" {
		t.Errorf("net_worth type: got %v", nwFS)
	}
	if !nwFS.Required {
		t.Error("net_worth should be required")
	}

	curFS := props["currency"]
	if curFS == nil || curFS.Default != "USD" {
		t.Errorf("currency default: got %v", curFS)
	}

	req := s.RequiredFields()
	if len(req) != 1 || req[0] != "net_worth" {
		t.Errorf("required fields: got %v", req)
	}
}

func TestSchemaFromJSON_Invalid(t *testing.T) {
	_, err := SchemaFromJSON(`not json`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSchemaToJSON(t *testing.T) {
	s, err := SchemaFromType[simpleStruct]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, err := s.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}
	if out == "" {
		t.Error("ToJSON returned empty string")
	}
}
