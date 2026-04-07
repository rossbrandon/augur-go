package augur

import "context"

// Provider is the abstraction boundary between Augur and LLM backends.
// Implementations handle authentication, API communication, and response
// extraction for a specific LLM service.
//
// Implementations must be safe for concurrent use.
type Provider interface {
	// Execute sends a prompt to the LLM and returns the raw text response.
	Execute(ctx context.Context, params *ProviderParams) (*ProviderResult, error)

	// Name returns a stable identifier for this provider (e.g., "claude", "openai").
	Name() string
}

// ProviderParams contains everything a provider needs to make an LLM call.
// These are provider-agnostic; each implementation maps them to its API.
type ProviderParams struct {
	SystemPrompt string
	UserPrompt   string
	// Model is the model identifier to use. Provider implementations
	// should define their own default if this is empty.
	Model       string
	Temperature float64
	MaxTokens   int
	// Sources, when non-nil, instructs the provider to enable web search
	// so that source citations are grounded in real search results.
	Sources *SourceConfig
}

// ProviderResult is the raw response from an LLM provider.
type ProviderResult struct {
	// Content is the raw text content of the LLM response, expected to be JSON.
	Content string
	// Model is the actual model used (may differ from requested if the provider
	// performed fallback).
	Model string
	Usage *Usage
}

// Usage tracks token consumption for cost monitoring.
type Usage struct {
	InputTokens       int `json:"inputTokens"`
	OutputTokens      int `json:"outputTokens"`
	WebSearchRequests int `json:"webSearchRequests,omitempty"`
}
