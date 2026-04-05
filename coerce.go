package augur

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// numericRe extracts the numeric portion of a string: optional sign, digits
// with optional commas, and optional decimal.
var numericRe = regexp.MustCompile(`-?[\d,]+(?:\.\d+)?`)

var naturalMagnitudes = []struct {
	word   string
	factor float64
}{
	{"trillion", 1e12},
	{"billion", 1e9},
	{"million", 1e6},
	{"thousand", 1e3},
}

// coerceValue attempts to coerce value v to the type described by fs.
// Returns the coerced value on success, or an error describing why it failed.
func coerceValue(v any, fs *fieldSchema) (any, error) {
	if v == nil {
		return nil, fmt.Errorf("value is null")
	}

	switch fs.Type {
	case "string":
		return coerceToString(v)
	case "integer":
		return coerceToInteger(v)
	case "number":
		return coerceToFloat(v)
	case "boolean":
		return coerceToBoolean(v)
	case "array":
		return coerceToArray(v, fs)
	case "object":
		// Objects are passed through as-is; nested coercion is handled recursively.
		if m, ok := v.(map[string]any); ok {
			return m, nil
		}
		return nil, fmt.Errorf("expected object, got %T", v)
	default:
		return v, nil
	}
}

// coerceToString converts a value to a string.
func coerceToString(v any) (any, error) {
	switch val := v.(type) {
	case string:
		return val, nil
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64), nil
	case int:
		return strconv.Itoa(val), nil
	case bool:
		return strconv.FormatBool(val), nil
	default:
		return nil, fmt.Errorf("cannot convert %T to string", v)
	}
}

// coerceToInteger converts a value to an integer (represented as float64 in JSON).
func coerceToInteger(v any) (any, error) {
	switch val := v.(type) {
	case float64:
		if math.Trunc(val) != val {
			return nil, fmt.Errorf("cannot convert non-integer %v to integer", val)
		}
		return val, nil
	case int:
		return float64(val), nil
	case string:
		n, err := parseNumericString(val)
		if err != nil {
			return nil, fmt.Errorf("cannot convert %q to integer: %w", val, err)
		}
		return n, nil
	case bool:
		if val {
			return float64(1), nil
		}
		return float64(0), nil
	default:
		return nil, fmt.Errorf("cannot convert %T to integer", v)
	}
}

// coerceToFloat converts a value to a float64.
func coerceToFloat(v any) (any, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case string:
		n, err := parseNumericString(val)
		if err != nil {
			return nil, fmt.Errorf("cannot convert %q to number: %w", val, err)
		}
		return n, nil
	default:
		return nil, fmt.Errorf("cannot convert %T to number", v)
	}
}

// coerceToBoolean converts a value to a bool.
func coerceToBoolean(v any) (any, error) {
	switch val := v.(type) {
	case bool:
		return val, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(val)) {
		case "true", "yes", "1":
			return true, nil
		case "false", "no", "0":
			return false, nil
		default:
			return nil, fmt.Errorf("cannot convert %q to boolean", val)
		}
	case float64:
		return val != 0, nil
	default:
		return nil, fmt.Errorf("cannot convert %T to boolean", v)
	}
}

// coerceToArray converts a value to a JSON array ([]any).
// If the value is a scalar (not already a slice), it is wrapped in a
// single-element array (e.g., "Rita Wilson" → ["Rita Wilson"]).
func coerceToArray(v any, fs *fieldSchema) (any, error) {
	switch val := v.(type) {
	case []any:
		if fs.Items == nil {
			return val, nil
		}
		// Coerce each element to the item type.
		result := make([]any, len(val))
		for i, elem := range val {
			coerced, err := coerceValue(elem, fs.Items)
			if err != nil {
				return nil, fmt.Errorf("array element %d: %w", i, err)
			}
			result[i] = coerced
		}
		return result, nil
	case nil:
		return nil, fmt.Errorf("value is null")
	default:
		// Scalar → single-element array.
		if fs.Items != nil {
			coerced, err := coerceValue(v, fs.Items)
			if err != nil {
				return nil, fmt.Errorf("cannot wrap scalar in array: %w", err)
			}
			return []any{coerced}, nil
		}
		return []any{v}, nil
	}
}

// parseNumericString parses a string that may represent a number with common
// LLM formatting: commas, currency symbols, magnitude suffixes (K/M/B/T),
// natural language magnitudes ("million", "billion"), and qualifiers
// ("approximately", "about", "~").
func parseNumericString(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}

	lower := strings.ToLower(s)

	// Detect natural language magnitude anywhere in the string.
	multiplier := 1.0
	for _, nm := range naturalMagnitudes {
		if strings.Contains(lower, nm.word) {
			multiplier = nm.factor
			break
		}
	}

	// Extract the numeric portion using regex.
	match := numericRe.FindString(s)
	if match == "" {
		return 0, fmt.Errorf("no numeric content in %q", s)
	}

	// Detect single-character magnitude suffix immediately after the number
	// (K/M/B/T), but only if no natural language magnitude was found.
	if multiplier == 1.0 {
		idx := strings.Index(s, match)
		after := strings.TrimSpace(s[idx+len(match):])
		if len(after) > 0 {
			switch unicode.ToUpper(rune(after[0])) {
			case 'K':
				multiplier = 1e3
			case 'M':
				multiplier = 1e6
			case 'B':
				multiplier = 1e9
			case 'T':
				multiplier = 1e12
			}
		}
	}

	// Strip commas from the matched number and parse.
	clean := strings.ReplaceAll(match, ",", "")
	n, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, fmt.Errorf("no numeric content in %q", s)
	}

	result := n * multiplier
	if math.IsInf(result, 0) || math.IsNaN(result) {
		return 0, fmt.Errorf("numeric overflow")
	}
	return result, nil
}
