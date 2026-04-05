package augur

import (
	"fmt"
	"strings"
)

const systemPromptTemplate = `You are a structured data retrieval engine. Your role is to answer queries by returning factual, well-sourced data in a precise JSON format.

RESPONSE FORMAT:
Return ONLY valid JSON with these exact top-level keys:

{
  "data": <object matching the provided schema>,
  "meta": {
    "<field_name>": {
      "confidence": <float 0.0-1.0>,
      "sources": [{"url": "<url>", "title": "<source name>"}]
    }
  },
  "notes": "<any caveats about data accuracy, completeness, or conflicts>"
}

RULES:
- "data" MUST conform exactly to the provided JSON Schema.
- Use the exact field names and types specified in the schema.
- For numeric fields, return raw numbers (not formatted strings like "400,000,000" or "$400M").
- If you cannot determine a value for an optional field, set it to null.
- If you cannot determine a value for a required field, still include it with your best estimate and set confidence below 0.5.
- "confidence" reflects your certainty: 1.0 = verified fact, 0.7-0.9 = high confidence, 0.4-0.6 = moderate, below 0.4 = low.
- "sources" should reference real, verifiable URLs when possible. Use an empty array if no specific sources are available.
- "meta" must contain an entry for every field present in "data".
- "notes" should mention any data conflicts, staleness, or relevant caveats.
- Return ONLY the JSON object. No markdown fences, no explanation, no preamble.`

// buildUserPrompt constructs the user-facing prompt from the request and schema.
func buildUserPrompt(req *Request, schema *Schema) (string, error) {
	schemaJSON, err := schema.ToJSON()
	if err != nil {
		return "", fmt.Errorf("serializing schema: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("Query: ")
	sb.WriteString(req.Query)

	if req.Context != "" {
		sb.WriteString("\n\nAdditional context: ")
		sb.WriteString(req.Context)
	}

	sb.WriteString("\n\nOutput JSON Schema:\n")
	sb.WriteString(schemaJSON)

	return sb.String(), nil
}

// buildRetryPrompt constructs a surgical retry prompt targeting only the
// fields that failed validation in the previous attempt.
func buildRetryPrompt(req *Request, failedFields []FieldError, schema *Schema) string {
	var sb strings.Builder

	sb.WriteString("Your previous response had issues with the following required fields:\n\n")

	for _, fe := range failedFields {
		fs := schema.properties[fe.Field]
		fieldType := "unknown"
		if fs != nil {
			fieldType = fs.Type
		}
		fmt.Fprintf(&sb, "- %q (required, type: %s): %s\n", fe.Field, fieldType, fe.Reason)
	}

	sb.WriteString("\nOriginal query: ")
	sb.WriteString(req.Query)

	if req.Context != "" {
		sb.WriteString("\n\nAdditional context: ")
		sb.WriteString(req.Context)
	}

	sb.WriteString("\n\nReturn ONLY the corrected fields as a JSON object wrapped in the standard response envelope:\n")
	sb.WriteString(`{"data": {<corrected fields only>}, "meta": {<meta for corrected fields>}, "notes": ""}`)
	sb.WriteString("\n\nFocus specifically on the fields listed above. Do not include fields that were already successfully populated.")

	return sb.String()
}
