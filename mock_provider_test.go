package augur_test

import (
	"context"
	"fmt"

	augur "github.com/rossbrandon/augur-go"
)

// mockProvider returns canned responses for deterministic unit testing.
type mockProvider struct {
	// responses is consumed in order; the last entry is repeated if exhausted.
	responses []string
	calls     int
	err       error
	// errOnCall, if non-nil, returns an error on the Nth call (0-indexed).
	errOnCall map[int]error
	// lastParams captures the most recent ProviderParams for assertions.
	lastParams *augur.ProviderParams
	// usage overrides the default usage returned by Execute.
	usage *augur.Usage
}

func newMock(responses ...string) *mockProvider {
	return &mockProvider{responses: responses}
}

func newMockErr(err error) *mockProvider {
	return &mockProvider{err: err}
}

func (m *mockProvider) Execute(_ context.Context, params *augur.ProviderParams) (*augur.ProviderResult, error) {
	callIdx := m.calls
	m.calls++
	m.lastParams = params

	if m.err != nil {
		return nil, m.err
	}
	if m.errOnCall != nil {
		if err, ok := m.errOnCall[callIdx]; ok {
			return nil, err
		}
	}

	idx := callIdx
	if idx >= len(m.responses) {
		idx = len(m.responses) - 1
	}

	usage := &augur.Usage{InputTokens: 10, OutputTokens: 20}
	if m.usage != nil {
		usage = m.usage
	}

	return &augur.ProviderResult{
		Content: m.responses[idx],
		Model:   "mock",
		Usage:   usage,
	}, nil
}

func (m *mockProvider) Name() string { return "mock" }

// envelope builds a minimal valid LLM response envelope JSON string.
func envelope(dataJSON string) string {
	return fmt.Sprintf(`{"data":%s,"meta":{},"notes":""}`, dataJSON)
}

// envelopeWithMeta builds an LLM response envelope with per-field meta data.
func envelopeWithMeta(dataJSON, metaJSON string) string {
	return fmt.Sprintf(`{"data":%s,"meta":%s,"notes":"test notes"}`, dataJSON, metaJSON)
}
