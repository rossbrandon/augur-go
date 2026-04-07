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

	// Sources enables source citations backed by web search. When non-nil,
	// web search is activated and FieldMeta.Sources will contain real URLs
	// from search results. When nil (the default), sources are always empty
	// and no web search cost is incurred.
	//
	// Enabling sources activates web search, which incurs additional cost
	// and increases token usage from search result content.
	// Not all models support web search; if the requested model is incompatible,
	// Query returns ErrSourcesNotSupported.
	Sources *SourceConfig
}

// SourceConfig configures source citations backed by web search.
// Enabling sources activates web search, which incurs additional cost
// and increases token usage from search result content.
// Sources are only populated when this config is provided.
type SourceConfig struct {
	// MaxSearches limits the number of web searches per query.
	// Nil defaults to 2. Higher values increase cost and token usage.
	MaxSearches *int

	// AllowedDomains restricts search results to these domains only.
	AllowedDomains []string

	// BlockedDomains excludes search results from these domains.
	BlockedDomains []string
}
