package augur

// Response wraps the query result with metadata and error information.
// T is the caller's output type.
//
// Response follows a GraphQL-inspired partial failure model:
//   - Full success:    Data is non-nil, Errors is empty
//   - Partial failure: Data is non-nil with nil/zero fields, Errors describes failures
//   - Total failure:   Data is nil, Errors is populated
type Response[T any] struct {
	// Data is the structured result matching the caller's schema.
	// Nil on total failure. Fields that could not be populated are set to
	// their zero/nil value.
	Data *T `json:"data"`

	// Meta contains per-field metadata keyed by top-level JSON field name.
	// Only includes entries for successfully populated fields. Nil on total failure.
	Meta map[string]*FieldMeta `json:"meta,omitempty"`

	// Errors describes fields that could not be populated.
	// Empty on full success. Non-empty on partial or total failure.
	Errors []FieldError `json:"errors,omitempty"`

	// Notes is free-text commentary from the LLM about data quality, caveats,
	// or additional context.
	Notes string `json:"notes,omitempty"`

	// Provider is the identifier of the provider used (e.g., "claude").
	Provider string `json:"provider"`

	// Model is the specific model identifier used for this query.
	Model string `json:"model"`

	// RetriesExecuted is the number of retries performed (0 on first-shot success).
	RetriesExecuted int `json:"retriesExecuted"`

	// LatencyMS is the total round-trip time in milliseconds, including retries.
	LatencyMS int64 `json:"latencyMs"`

	// Usage contains token consumption metrics from the provider.
	Usage *Usage `json:"usage,omitempty"`
}

// FieldMeta contains metadata about a single successfully populated field.
type FieldMeta struct {
	// Confidence is the LLM's self-assessed confidence, from 0.0 to 1.0.
	Confidence float64 `json:"confidence"`

	// Sources is a list of citations supporting this field's value.
	Sources []Source `json:"sources,omitempty"`
}

// Source represents a citation for a piece of data.
type Source struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// OK reports whether the query was a full success: all fields populated with no errors.
func (r *Response[T]) OK() bool { return r.Data != nil && len(r.Errors) == 0 }

// IsPartial reports whether the query returned usable data but some optional
// fields could not be populated. Check r.Errors for details.
func (r *Response[T]) IsPartial() bool { return r.Data != nil && len(r.Errors) > 0 }

// FieldError describes why a specific field could not be populated.
type FieldError struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}
