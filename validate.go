package augur

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// llmEnvelope is the expected top-level structure of every LLM response.
type llmEnvelope struct {
	Data  map[string]any            `json:"data"`
	Meta  map[string]map[string]any `json:"meta"`
	Notes string                    `json:"notes"`
}

// processResult holds the outcome of a single provider call after validation.
type processResult struct {
	data   map[string]any
	meta   map[string]map[string]any
	notes  string
	errors []FieldError // field-level failures from this pass
}

var markdownFenceRe = regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(.*?)\\n?```")

// processResponse runs the full validation pipeline on a raw LLM response string:
// JSON extraction → envelope validation → coercion pass → required field check.
func processResponse(raw string, schema *Schema, logger *slog.Logger) (*processResult, error) {
	// Step 1: Extract and parse the envelope in a single pass.
	env, err := extractAndParseEnvelope(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrResponseMalformed, err)
	}

	if env.Data == nil {
		return nil, fmt.Errorf("%w: response envelope missing \"data\" key", ErrResponseMalformed)
	}

	// Step 2: Coercion pass — walk every schema field and coerce.
	result := &processResult{
		data:  make(map[string]any),
		meta:  env.Meta,
		notes: env.Notes,
	}

	for fieldName, fs := range schema.properties {
		val, present := env.Data[fieldName]

		if !present || val == nil {
			if fs.Default != nil {
				if logger != nil {
					logger.Debug("applying default value", "field", fieldName, "default", fs.Default)
				}
				coerced, err := coerceValue(fs.Default, fs)
				if err == nil {
					result.data[fieldName] = coerced
					continue
				}
			}
			reason := "field not present in response"
			if present {
				reason = "field is null"
			}
			result.errors = append(result.errors, FieldError{Field: fieldName, Reason: reason})
			continue
		}

		coerced, err := coerceValue(val, fs)
		if err != nil {
			if logger != nil {
				logger.Debug("coercion failed", "field", fieldName, "type", fs.Type, "value", val, "error", err)
			}
			result.errors = append(result.errors, FieldError{
				Field:  fieldName,
				Reason: fmt.Sprintf("type coercion failed (expected %s): %s", fs.Type, err),
			})
			continue
		}

		if logger != nil {
			logger.Debug("coerced value", "field", fieldName, "from", val, "to", coerced)
		}
		result.data[fieldName] = coerced
	}

	return result, nil
}

// requiredFailures returns only the FieldErrors from result that correspond to
// required fields in schema.
func requiredFailures(result *processResult, schema *Schema) []FieldError {
	var failed []FieldError
	props := schema.properties
	for _, fe := range result.errors {
		if fs, ok := props[fe.Field]; ok && fs.Required {
			failed = append(failed, fe)
		}
	}
	return failed
}

// mergeRetryResult merges fields from a retry processResult into the base result.
// Only fields present in the retry data are merged; already-successful fields in
// base are preserved.
func mergeRetryResult(base, retry *processResult) {
	for k, v := range retry.data {
		base.data[k] = v
	}

	// Remove errors for fields that were successfully resolved in the retry.
	resolved := make(map[string]bool, len(retry.data))
	for k := range retry.data {
		resolved[k] = true
	}

	filtered := base.errors[:0]
	for _, fe := range base.errors {
		if !resolved[fe.Field] {
			filtered = append(filtered, fe)
		}
	}
	base.errors = filtered

	// Merge meta.
	if base.meta == nil {
		base.meta = make(map[string]map[string]any)
	}
	for k, v := range retry.meta {
		base.meta[k] = v
	}

	if retry.notes != "" && base.notes == "" {
		base.notes = retry.notes
	}
}

// buildResponseMeta converts the raw meta map from the LLM envelope into the
// typed map[string]*FieldMeta used in Response.
func buildResponseMeta(raw map[string]map[string]any) map[string]*FieldMeta {
	if len(raw) == 0 {
		return nil
	}
	result := make(map[string]*FieldMeta, len(raw))
	for field, m := range raw {
		fm := &FieldMeta{}
		if c, ok := m["confidence"].(float64); ok {
			fm.Confidence = c
		}
		if srcs, ok := m["sources"].([]any); ok {
			for _, s := range srcs {
				if sm, ok := s.(map[string]any); ok {
					src := Source{}
					if u, ok := sm["url"].(string); ok {
						src.URL = u
					}
					if t, ok := sm["title"].(string); ok {
						src.Title = t
					}
					if ct, ok := sm["citedText"].(string); ok {
						src.CitedText = ct
					}
					fm.Sources = append(fm.Sources, src)
				}
			}
		}
		result[field] = fm
	}
	return result
}

// extractAndParseEnvelope attempts to extract and parse an llmEnvelope from a
// string that may contain non-JSON content (markdown fences, preamble, etc.).
// It tries three strategies in order:
//  1. Direct parse
//  2. Strip markdown code fences
//  3. Brace scanning (first '{' to last '}')
func extractAndParseEnvelope(s string) (*llmEnvelope, error) {
	s = strings.TrimSpace(s)

	// Strategy 1: direct parse.
	if env, ok := tryParseEnvelope(s); ok {
		return env, nil
	}

	// Strategy 2: strip markdown fences.
	if stripped, ok := stripMarkdownFences(s); ok {
		if env, ok := tryParseEnvelope(stripped); ok {
			return env, nil
		}
	}

	// Strategy 3: brace scan.
	if scanned, ok := scanBraces(s); ok {
		if env, ok := tryParseEnvelope(scanned); ok {
			return env, nil
		}
	}

	preview := s
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}
	return nil, fmt.Errorf("could not extract JSON from response: %q", preview)
}

// tryParseEnvelope attempts to unmarshal s into an llmEnvelope.
func tryParseEnvelope(s string) (*llmEnvelope, bool) {
	var env llmEnvelope
	if err := json.Unmarshal([]byte(s), &env); err != nil {
		return nil, false
	}
	return &env, true
}

// stripMarkdownFences removes ```json ... ``` or ``` ... ``` fences.
func stripMarkdownFences(s string) (string, bool) {
	matches := markdownFenceRe.FindStringSubmatch(s)
	if len(matches) < 2 {
		return "", false
	}
	return strings.TrimSpace(matches[1]), true
}

// scanBraces extracts the substring from the first '{' to the last '}'.
func scanBraces(s string) (string, bool) {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end <= start {
		return "", false
	}
	return s[start : end+1], true
}
