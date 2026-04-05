package augur

import (
	"testing"
)

func TestParseNumericString(t *testing.T) {
	tests := []struct {
		input   string
		want    float64
		wantErr bool
	}{
		// Direct numeric strings
		{"400000000", 400000000, false},
		{"3.14", 3.14, false},
		{"0", 0, false},

		// Comma-separated
		{"400,000,000", 400000000, false},
		{"1,234.56", 1234.56, false},

		// Currency symbols
		{"$400,000,000", 400000000, false},
		{"€3.5", 3.5, false},
		{"£1,000", 1000, false},

		// Single-char magnitude suffixes
		{"400M", 400000000, false},
		{"1.5B", 1500000000, false},
		{"500K", 500000, false},
		{"2T", 2000000000000, false},

		// Natural language magnitudes
		{"400 million", 400000000, false},
		{"1.5 billion", 1500000000, false},
		{"500 thousand", 500000, false},
		{"2 trillion", 2000000000000, false},

		// Qualifiers
		{"approximately 400 million", 400000000, false},
		{"about $1.5B", 1500000000, false},
		{"roughly 500K", 500000, false},
		{"~$400M", 400000000, false},

		// Currency + magnitude
		{"$3.5B", 3500000000, false},
		{"$400M", 400000000, false},

		// Failure cases
		{"unknown", 0, true},
		{"", 0, true},
		{"not a number", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseNumericString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseNumericString(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseNumericString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCoerceToBoolean(t *testing.T) {
	tests := []struct {
		input   any
		want    any
		wantErr bool
	}{
		{true, true, false},
		{false, false, false},
		{"true", true, false},
		{"false", false, false},
		{"yes", true, false},
		{"no", false, false},
		{"1", true, false},
		{"0", false, false},
		{"TRUE", true, false},
		{"YES", true, false},
		{float64(1), true, false},
		{float64(0), false, false},
		{"maybe", nil, true},
		{"unknown", nil, true},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got, err := coerceToBoolean(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("coerceToBoolean(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("coerceToBoolean(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCoerceToArray_ScalarWrap(t *testing.T) {
	fs := &fieldSchema{Type: "array", Items: &fieldSchema{Type: "string"}}

	// Scalar string should be wrapped in a single-element array.
	got, err := coerceToArray("Rita Wilson", fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	arr, ok := got.([]any)
	if !ok || len(arr) != 1 || arr[0] != "Rita Wilson" {
		t.Errorf("expected [\"Rita Wilson\"], got %v", got)
	}
}

func TestCoerceToArray_ElementCoercion(t *testing.T) {
	fs := &fieldSchema{Type: "array", Items: &fieldSchema{Type: "integer"}}

	// Array of numeric strings should be coerced to integers.
	got, err := coerceToArray([]any{"1M", "2M", "3M"}, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	arr := got.([]any)
	want := []float64{1000000, 2000000, 3000000}
	for i, w := range want {
		if arr[i] != w {
			t.Errorf("element %d: got %v, want %v", i, arr[i], w)
		}
	}
}
