// Package augur is a model-agnostic Go library that uses LLMs as a structured,
// schema-aware data retrieval layer.
//
// # Overview
//
// Augur lets callers describe what data they want in natural language, define the
// expected output shape via a JSON Schema (or a Go struct), and receive a
// strongly-typed response with per-field provenance metadata.
//
// # Usage
//
//	// Define the output shape.
//	type Actor struct {
//		Name     string   `json:"name"     augur:"required,desc:Full name"`
//		NetWorth int64    `json:"netWorth"  augur:"required,desc:Estimated net worth in USD"`
//		Spouse   string   `json:"spouse"`
//	}
//
//	// Create a client backed by the Claude provider.
//	client := augur.New(claude.NewProvider(os.Getenv("ANTHROPIC_API_KEY")))
//
//	// Execute a structured query.
//	resp, err := augur.Query[Actor](ctx, client, &augur.Request{
//		Query: "Tom Hanks biography",
//	})
//
// # Error handling
//
// Augur uses two error channels:
//
//   - The returned Go error covers infrastructure failures only
//     (ErrProviderFailure, ErrResponseMalformed, ErrSchemaInvalid).
//     When err is non-nil, resp is nil.
//
//   - resp (always non-nil when err is nil) carries data-level outcomes.
//     resp.Data == nil means total failure (required fields unresolved);
//     check resp.Errors for details. resp.Data != nil means full or partial
//     success; check resp.Errors for any optional fields that failed.
//
// # Schema definition
//
// Schemas can be provided three ways:
//
//  1. Struct reflection via augur struct tags (SchemaFromType[T]):
//     `augur:"required"`, `augur:"desc:text"`, `augur:"default:val"`
//
//  2. Inline JSON string (SchemaFromJSON)
//
//  3. File path (SchemaFromFile)
//
// When req.Schema is set explicitly it takes precedence over T.
package augur
