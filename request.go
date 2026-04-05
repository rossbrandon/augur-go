package augur

// Request defines what data to retrieve and how.
type Request struct {
	// Query is the natural language description of the data to retrieve.
	Query string

	// Schema is the JSON Schema defining the expected output shape.
	// When nil, the schema is derived from the generic type parameter T via
	// reflection. When provided explicitly (via SchemaFromJSON or SchemaFromFile),
	// it takes precedence over reflection.
	//
	// A schema is always required — the library never infers one on its own.
	// Returns ErrSchemaInvalid if neither T yields a valid schema nor an
	// explicit Schema is provided.
	Schema *Schema

	// Context is optional additional context injected into the LLM prompt to
	// improve result quality (e.g., "Focus on the actor's personal life").
	Context string

	// Options are per-query configuration overrides for client defaults.
	Options *QueryOptions
}

// QueryOptions provides per-query overrides for client-level defaults.
type QueryOptions struct {
	// Model overrides the client's default model for this query.
	Model string

	// Temperature controls LLM randomness. Default: 0.0 (deterministic).
	Temperature *float64

	// MaxTokens overrides the client's default max output tokens for this query.
	MaxTokens *int
}
