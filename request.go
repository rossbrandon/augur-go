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

	// Sources configures web search and source citations for this query.
	// When nil, the source configuration defined at the client level is used 
	// (web search is enabled by default).
	// Provide a SourceConfig to customize web search behavior 
	// or set Disabled: true to turn off web search for this query.
	//
	// Not all models support web search; if the requested model is incompatible,
	// Query returns ErrSourcesNotSupported.
	Sources *SourceConfig
}

// SourceConfig configures source citations backed by web search.
// Web search is enabled by default. Set Disabled to true to turn it off,
// which avoids web search cost and returns empty FieldMeta.Sources.
type SourceConfig struct {
	// Disabled turns off web search entirely when true. When false (the
	// default), web search is active and FieldMeta.Sources will contain
	// real URLs from search results.
	Disabled bool

	// MaxSearches limits the number of web searches per query.
	// Nil defaults to 2. Higher values increase cost and token usage.
	MaxSearches *int

	// AllowedDomains restricts search results to these domains only.
	AllowedDomains []string

	// BlockedDomains excludes search results from these domains.
	BlockedDomains []string
}
