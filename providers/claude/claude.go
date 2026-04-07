// Package claude provides an Augur provider implementation for the Anthropic
// Claude API using the official Anthropic Go SDK.
package claude

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	augur "github.com/rossbrandon/augur-go"
)

const (
	defaultModel       = "claude-sonnet-4-6"
	defaultMaxSearches = 2
)

// Provider implements augur.Provider for the Anthropic Claude API.
type Provider struct {
	client     anthropic.Client
	apiKey     string
	baseURL    string
	httpClient *http.Client
	model      string
}

// Compile-time interface verification.
var _ augur.Provider = (*Provider)(nil)

// Option configures the Claude provider.
type Option func(*Provider)

// WithModel sets the default model for this provider.
// Default: "claude-sonnet-4-6".
func WithModel(model string) Option {
	return func(p *Provider) {
		p.model = model
	}
}

// WithBaseURL overrides the Anthropic API base URL (useful for proxies or testing).
func WithBaseURL(url string) Option {
	return func(p *Provider) {
		p.baseURL = url
	}
}

// WithHTTPClient sets a custom HTTP client on the underlying Anthropic client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(p *Provider) {
		p.httpClient = httpClient
	}
}

// NewProvider creates a Claude provider with the given API key and options.
// If apiKey is empty, the SDK falls back to the ANTHROPIC_API_KEY environment
// variable.
func NewProvider(apiKey string, opts ...Option) *Provider {
	p := &Provider{
		apiKey: apiKey,
		model:  defaultModel,
	}
	for _, opt := range opts {
		opt(p)
	}
	p.client = buildClient(p.apiKey, p.baseURL, p.httpClient)
	return p
}

// Execute sends the prompt to the Claude API and returns the raw text response.
// The model in params takes precedence over the provider's default model.
func (p *Provider) Execute(ctx context.Context, params *augur.ProviderParams) (*augur.ProviderResult, error) {
	model := p.model
	if params.Model != "" {
		model = params.Model
	}

	msgParams := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(params.MaxTokens),
		System: []anthropic.TextBlockParam{
			{Text: params.SystemPrompt, Type: "text"},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(params.UserPrompt)),
		},
	}

	msgParams.Temperature = anthropic.Float(params.Temperature)

	if params.Sources != nil {
		msgParams.Tools = buildWebSearchTools(params.Sources)
	}

	msg, err := p.client.Messages.New(ctx, msgParams)
	if err != nil {
		if params.Sources != nil && isWebSearchNotSupported(err) {
			return nil, fmt.Errorf("%w: model %q: %w", augur.ErrSourcesNotSupported, model, err)
		}
		return nil, fmt.Errorf("claude API call failed: %w", err)
	}

	var content string
	for _, block := range msg.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			content += tb.Text
		}
	}

	usage := &augur.Usage{
		InputTokens:  int(msg.Usage.InputTokens),
		OutputTokens: int(msg.Usage.OutputTokens),
	}
	if params.Sources != nil {
		usage.WebSearchRequests = int(msg.Usage.ServerToolUse.WebSearchRequests)
	}

	return &augur.ProviderResult{
		Content: content,
		Model:   string(msg.Model),
		Usage:   usage,
	}, nil
}

func buildWebSearchTools(cfg *augur.SourceConfig) []anthropic.ToolUnionParam {
	maxSearches := int64(defaultMaxSearches)
	if cfg.MaxSearches != nil {
		maxSearches = int64(*cfg.MaxSearches)
	}

	searchTool := &anthropic.WebSearchTool20250305Param{
		MaxUses: anthropic.Int(maxSearches),
	}
	if len(cfg.AllowedDomains) > 0 {
		searchTool.AllowedDomains = cfg.AllowedDomains
	}
	if len(cfg.BlockedDomains) > 0 {
		searchTool.BlockedDomains = cfg.BlockedDomains
	}

	return []anthropic.ToolUnionParam{
		{OfWebSearchTool20250305: searchTool},
	}
}

func isWebSearchNotSupported(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "web_search") && (strings.Contains(msg, "not supported") || strings.Contains(msg, "not available"))
}

// Name returns the stable provider identifier.
func (p *Provider) Name() string { return "claude" }

// buildClient constructs an anthropic.Client with the given options.
func buildClient(apiKey, baseURL string, httpClient *http.Client) anthropic.Client {
	opts := []option.RequestOption{}
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	if httpClient != nil {
		opts = append(opts, option.WithHTTPClient(httpClient))
	}
	return anthropic.NewClient(opts...)
}
