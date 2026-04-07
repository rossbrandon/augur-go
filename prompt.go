package augur

import (
	"fmt"
	"strings"
)

const systemPromptBase = `You are a structured data retrieval engine. Your role is to answer queries by returning factual, well-sourced data in a precise JSON format.

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
- "meta" must contain an entry for every field present in "data".
- "notes" should mention any data conflicts, staleness, or relevant caveats.
- In "notes", never reference null fields, LLM training data, or knowledge cutoffs. Write as a knowledgeable human would:
    - Use "no known children at this time" instead of "'children' is null".
    - Use "as of 2024" instead of "as of the knowledge cutoff".
    - Focus only on meaningful observations about the data: conflicts between
      sources, uncertainty, missing information, or notable context.
- Return ONLY the JSON object. No markdown fences, no explanation, no preamble.`

const sourcesDisabledRule = `
- "sources" MUST always be an empty array. You do not have the ability to provide real source URLs. NEVER fabricate, guess, or reconstruct URLs under any circumstances.`

const sourcesEnabledRule = `
- You have web search tools available. Use them to find current, accurate data for your response.
- "sources" MUST only contain URLs returned by your web search results. Include the URL and title from the search result. NEVER fabricate URLs that did not come from a search result.`

// buildSystemPrompt constructs the system prompt with the appropriate sources
// instruction based on whether source citations are enabled for this query.
func buildSystemPrompt(sourcesEnabled bool) string {
	if sourcesEnabled {
		return systemPromptBase + sourcesEnabledRule
	}
	return systemPromptBase + sourcesDisabledRule
}

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
