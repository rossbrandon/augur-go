package augur

import (
	"strings"
	"testing"
)

func TestBuildSystemPrompt_SourcesDisabled(t *testing.T) {
	prompt := buildSystemPrompt(false)

	if !strings.Contains(prompt, `"sources" MUST always be an empty array`) {
		t.Error("sources-disabled prompt should forbid source URLs")
	}
	if !strings.Contains(prompt, "NEVER fabricate") {
		t.Error("sources-disabled prompt should contain anti-hallucination instruction")
	}
	if strings.Contains(prompt, "web search tools available") {
		t.Error("sources-disabled prompt should not mention web search tools")
	}
}

func TestBuildSystemPrompt_SourcesEnabled(t *testing.T) {
	prompt := buildSystemPrompt(true)

	if !strings.Contains(prompt, "web search tools available") {
		t.Error("sources-enabled prompt should mention web search tools")
	}
	if !strings.Contains(prompt, "MUST only contain URLs returned by your web search results") {
		t.Error("sources-enabled prompt should require URLs from search results")
	}
	if strings.Contains(prompt, `"sources" MUST always be an empty array`) {
		t.Error("sources-enabled prompt should not forbid sources")
	}
}

func TestBuildSystemPrompt_SharedBase(t *testing.T) {
	disabled := buildSystemPrompt(false)
	enabled := buildSystemPrompt(true)

	for _, keyword := range []string{
		"structured data retrieval engine",
		`"data" MUST conform exactly`,
		`"confidence" reflects your certainty`,
		`"meta" must contain an entry`,
	} {
		if !strings.Contains(disabled, keyword) {
			t.Errorf("sources-disabled prompt missing base content: %q", keyword)
		}
		if !strings.Contains(enabled, keyword) {
			t.Errorf("sources-enabled prompt missing base content: %q", keyword)
		}
	}
}
