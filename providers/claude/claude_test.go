package claude

import (
	"net/http"
	"testing"
)

func TestNewProvider_Defaults(t *testing.T) {
	p := NewProvider("test-key")

	if p.apiKey != "test-key" {
		t.Errorf("expected apiKey %q, got %q", "test-key", p.apiKey)
	}
	if p.model != defaultModel {
		t.Errorf("expected model %q, got %q", defaultModel, p.model)
	}
	if p.baseURL != "" {
		t.Errorf("expected empty baseURL, got %q", p.baseURL)
	}
	if p.httpClient != nil {
		t.Error("expected nil httpClient")
	}
}

func TestNewProvider_WithOptions(t *testing.T) {
	customClient := &http.Client{}

	p := NewProvider("key-2",
		WithModel("claude-haiku-4-5"),
		WithBaseURL("https://proxy.example.com"),
		WithHTTPClient(customClient),
	)

	if p.model != "claude-haiku-4-5" {
		t.Errorf("expected model %q, got %q", "claude-haiku-4-5", p.model)
	}
	if p.baseURL != "https://proxy.example.com" {
		t.Errorf("expected baseURL %q, got %q", "https://proxy.example.com", p.baseURL)
	}
	if p.httpClient != customClient {
		t.Error("expected custom httpClient to be set")
	}
}

func TestProvider_Name(t *testing.T) {
	p := NewProvider("")
	if name := p.Name(); name != "claude" {
		t.Errorf("expected name %q, got %q", "claude", name)
	}
}
