package augur

import "errors"

var (
	// ErrSchemaInvalid indicates the provided schema (or struct type) could not
	// be parsed into a valid JSON Schema.
	ErrSchemaInvalid = errors.New("augur: invalid schema")

	// ErrInvalidRequest indicates the request is malformed (nil, missing query, etc.).
	ErrInvalidRequest = errors.New("augur: invalid request")

	// ErrProviderFailure indicates the LLM provider returned an error
	// (network failure, auth failure, rate limit, etc.).
	ErrProviderFailure = errors.New("augur: provider execution failed")

	// ErrResponseMalformed indicates the LLM returned a response that could not
	// be parsed as valid JSON after all extraction attempts (direct parse,
	// markdown fence stripping, brace scanning). The raw content is included
	// in the wrapped error message for debugging.
	ErrResponseMalformed = errors.New("augur: could not parse provider response as JSON")
)
