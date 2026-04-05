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
}

func newMock(responses ...string) *mockProvider {
	return &mockProvider{responses: responses}
}

func newMockErr(err error) *mockProvider {
	return &mockProvider{err: err}
}

func (m *mockProvider) Execute(_ context.Context, _ *augur.ProviderParams) (*augur.ProviderResult, error) {
	callIdx := m.calls
	m.calls++

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
	return &augur.ProviderResult{
		Content: m.responses[idx],
		Model:   "mock",
		Usage:   &augur.Usage{InputTokens: 10, OutputTokens: 20},
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
