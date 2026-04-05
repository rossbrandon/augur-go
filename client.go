package augur

import "log/slog"

const (
	defaultMaxRetries = 2
	defaultMaxTokens  = 8192
)

// Client is the main entry point for Augur queries. It holds a provider and
// client-level configuration applied to every query unless overridden via
// QueryOptions.
type Client struct {
	provider   Provider
	maxRetries int
	maxTokens  int
	model      string
	logger     *slog.Logger
}

// Option configures a Client via the functional options pattern.
type Option func(*Client)

// WithMaxRetries sets the maximum number of retry attempts for required fields
// that fail validation. Default: 2.
func WithMaxRetries(n int) Option {
	return func(c *Client) {
		c.maxRetries = n
	}
}

// WithModel sets the model identifier for this client, overriding the provider's default.
// Can be further overridden per-query via QueryOptions.Model.
func WithModel(model string) Option {
	return func(c *Client) {
		c.model = model
	}
}

// WithMaxTokens sets the maximum number of output tokens for LLM responses.
// Default: 8192.
func WithMaxTokens(n int) Option {
	return func(c *Client) {
		c.maxTokens = n
	}
}

// WithLogger enables structured debug logging for internal operations (prompt
// construction, coercion decisions, retry attempts). Silent by default.
func WithLogger(logger *slog.Logger) Option {
	return func(c *Client) {
		c.logger = logger
	}
}

// New creates a new Augur client with the given provider and options.
// Panics if provider is nil.
func New(provider Provider, opts ...Option) *Client {
	if provider == nil {
		panic("augur: provider must not be nil")
	}
	c := &Client{
		provider:   provider,
		maxRetries: defaultMaxRetries,
		maxTokens:  defaultMaxTokens,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
