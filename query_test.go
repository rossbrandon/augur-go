package augur_test

import (
	"context"
	"errors"
	"testing"

	augur "github.com/rossbrandon/augur-go"
)

type actorFamily struct {
	Spouse   string   `json:"spouse"   augur:"required,desc:Current or most recent spouse"`
	Children []string `json:"children" augur:"required,desc:Biological and adopted children"`
	Parents  []string `json:"parents"  augur:"desc:Biological parents"`
}

type netWorth struct {
	Amount   int64  `json:"amount"   augur:"required,desc:Estimated net worth in USD"`
	Currency string `json:"currency" augur:"required,default:USD"`
}

func TestQuery_FullSuccess(t *testing.T) {
	resp := envelope(`{"spouse":"Rita Wilson","children":["Colin","Elizabeth"],"parents":["Amos","Janet"]}`)
	client := augur.New(newMock(resp))

	result, err := augur.Query[actorFamily](context.Background(), client, &augur.Request{
		Query: "Tom Hanks family",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data == nil {
		t.Fatal("Data should not be nil on full success")
	}
	if result.Data.Spouse != "Rita Wilson" {
		t.Errorf("spouse: got %q", result.Data.Spouse)
	}
	if len(result.Data.Children) != 2 {
		t.Errorf("children count: got %d", len(result.Data.Children))
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
}

func TestQuery_PartialSuccess_OptionalFieldMissing(t *testing.T) {
	// Parents is optional — missing it should yield partial success (err == nil).
	resp := envelope(`{"spouse":"Rita Wilson","children":["Colin","Elizabeth"]}`)
	client := augur.New(newMock(resp))

	result, err := augur.Query[actorFamily](context.Background(), client, &augur.Request{
		Query: "Tom Hanks family",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data == nil {
		t.Fatal("Data should not be nil on partial success")
	}
	if len(result.Errors) == 0 {
		t.Error("expected field errors for missing optional field")
	}
	found := false
	for _, e := range result.Errors {
		if e.Field == "parents" {
			found = true
		}
	}
	if !found {
		t.Error("expected parents in field errors")
	}
}

func TestQuery_TotalFailure_RequiredFieldMissing(t *testing.T) {
	// Spouse is required — missing after retries yields total failure.
	// err is nil; total failure is signaled by resp.Data == nil.
	resp := envelope(`{"children":["Colin","Elizabeth"]}`)
	client := augur.New(newMock(resp), augur.WithMaxRetries(0))

	result, err := augur.Query[actorFamily](context.Background(), client, &augur.Request{
		Query: "Tom Hanks family",
	})
	if err != nil {
		t.Fatalf("expected nil error on total failure, got %v", err)
	}
	if result == nil {
		t.Fatal("result should be non-nil on total failure")
	}
	if result.Data != nil {
		t.Error("Data should be nil on total failure")
	}
	if len(result.Errors) == 0 {
		t.Error("expected field errors on total failure")
	}
}

func TestQuery_Retry_ResolvesRequiredField(t *testing.T) {
	// First call missing spouse; second (retry) provides it.
	first := envelope(`{"children":["Colin","Elizabeth"]}`)
	second := envelope(`{"spouse":"Rita Wilson"}`)
	client := augur.New(newMock(first, second), augur.WithMaxRetries(1))

	result, err := augur.Query[actorFamily](context.Background(), client, &augur.Request{
		Query: "Tom Hanks family",
	})
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}
	if result.Data == nil {
		t.Fatal("Data should not be nil after successful retry")
	}
	if result.Data.Spouse != "Rita Wilson" {
		t.Errorf("spouse after retry: got %q", result.Data.Spouse)
	}
}

func TestQuery_DefaultApplied(t *testing.T) {
	resp := envelope(`{"amount":400000000}`)
	client := augur.New(newMock(resp))

	result, err := augur.Query[netWorth](context.Background(), client, &augur.Request{
		Query: "Tom Hanks net worth",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data.Currency != "USD" {
		t.Errorf("default currency: got %q, want %q", result.Data.Currency, "USD")
	}
}

func TestQuery_CoercesStringToInt(t *testing.T) {
	resp := envelope(`{"amount":"$400,000,000","currency":"USD"}`)
	client := augur.New(newMock(resp))

	result, err := augur.Query[netWorth](context.Background(), client, &augur.Request{
		Query: "Tom Hanks net worth",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data.Amount != 400000000 {
		t.Errorf("amount after coercion: got %d, want 400000000", result.Data.Amount)
	}
}

func TestQuery_ProviderFailure(t *testing.T) {
	client := augur.New(newMockErr(errors.New("connection refused")))

	_, err := augur.Query[actorFamily](context.Background(), client, &augur.Request{
		Query: "Tom Hanks family",
	})
	if !errors.Is(err, augur.ErrProviderFailure) {
		t.Fatalf("expected ErrProviderFailure, got %v", err)
	}
}

func TestQuery_MalformedResponse(t *testing.T) {
	client := augur.New(newMock("this is not json"))

	_, err := augur.Query[actorFamily](context.Background(), client, &augur.Request{
		Query: "Tom Hanks family",
	})
	if !errors.Is(err, augur.ErrResponseMalformed) {
		t.Fatalf("expected ErrResponseMalformed, got %v", err)
	}
}

func TestQuery_ExplicitSchema(t *testing.T) {
	const schemaJSON = `{
		"type": "object",
		"properties": {
			"net_worth": {"type": "integer", "description": "Net worth in USD"}
		},
		"required": ["net_worth"]
	}`
	schema, err := augur.SchemaFromJSON(schemaJSON)
	if err != nil {
		t.Fatalf("SchemaFromJSON: %v", err)
	}

	resp := envelope(`{"net_worth":400000000}`)
	client := augur.New(newMock(resp))

	result, err := augur.Query[map[string]any](context.Background(), client, &augur.Request{
		Query:  "Tom Hanks net worth",
		Schema: schema,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data == nil {
		t.Fatal("expected non-nil Data")
	}
	nw, ok := (*result.Data)["net_worth"]
	if !ok {
		t.Fatal("net_worth missing from result")
	}
	if nw != float64(400000000) {
		t.Errorf("net_worth: got %v", nw)
	}
}

func TestQuery_NoSchema_MapType_ReturnsError(t *testing.T) {
	client := augur.New(newMock(`{}`))

	_, err := augur.Query[map[string]any](context.Background(), client, &augur.Request{
		Query: "anything",
	})
	if !errors.Is(err, augur.ErrSchemaInvalid) {
		t.Fatalf("expected ErrSchemaInvalid, got %v", err)
	}
}

func TestQuery_TokenUsageAccumulated(t *testing.T) {
	// Two calls (initial + retry), each returning 10 input / 20 output tokens.
	first := envelope(`{"children":["Colin"]}`)
	second := envelope(`{"spouse":"Rita Wilson"}`)
	client := augur.New(newMock(first, second), augur.WithMaxRetries(1))

	result, err := augur.Query[actorFamily](context.Background(), client, &augur.Request{
		Query: "Tom Hanks family",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Usage == nil {
		t.Fatal("expected non-nil Usage")
	}
	if result.Usage.InputTokens != 20 {
		t.Errorf("input tokens: got %d, want 20", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 40 {
		t.Errorf("output tokens: got %d, want 40", result.Usage.OutputTokens)
	}
}

func TestQuery_MetaFlowsThrough(t *testing.T) {
	resp := envelopeWithMeta(
		`{"spouse":"Rita Wilson","children":["Colin","Elizabeth"],"parents":["Amos"]}`,
		`{"spouse":{"confidence":0.99,"sources":[{"url":"https://example.com","title":"Wikipedia"}]},"children":{"confidence":0.9,"sources":[]}}`,
	)
	client := augur.New(newMock(resp))

	result, err := augur.Query[actorFamily](context.Background(), client, &augur.Request{
		Query: "Tom Hanks family",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Meta == nil {
		t.Fatal("expected non-nil Meta")
	}
	spouseMeta, ok := result.Meta["spouse"]
	if !ok {
		t.Fatal("expected spouse in Meta")
	}
	if spouseMeta.Confidence != 0.99 {
		t.Errorf("spouse confidence: got %v, want 0.99", spouseMeta.Confidence)
	}
	if len(spouseMeta.Sources) != 1 {
		t.Fatalf("spouse sources count: got %d, want 1", len(spouseMeta.Sources))
	}
	if spouseMeta.Sources[0].URL != "https://example.com" {
		t.Errorf("spouse source url: got %q", spouseMeta.Sources[0].URL)
	}
	if spouseMeta.Sources[0].Title != "Wikipedia" {
		t.Errorf("spouse source title: got %q", spouseMeta.Sources[0].Title)
	}
	if result.Notes != "test notes" {
		t.Errorf("notes: got %q, want %q", result.Notes, "test notes")
	}
}

func TestQuery_CancelledContext_DuringRetry(t *testing.T) {
	// First call missing required field triggers retry, but context is
	// cancelled before the retry executes.
	ctx, cancel := context.WithCancel(context.Background())

	mock := &mockProvider{
		responses: []string{envelope(`{"children":["Colin"]}`)},
	}
	client := augur.New(mock, augur.WithMaxRetries(1))

	// Cancel after the first call completes (before retry).
	cancel()

	_, err := augur.Query[actorFamily](ctx, client, &augur.Request{
		Query: "Tom Hanks family",
	})
	if err == nil {
		t.Fatal("expected error for cancelled context during retry")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestQuery_ProviderFailureOnRetry(t *testing.T) {
	// First call succeeds but missing required field; retry fails with provider error.
	mock := &mockProvider{
		responses: []string{envelope(`{"children":["Colin"]}`)},
		errOnCall: map[int]error{1: errors.New("rate limited")},
	}
	client := augur.New(mock, augur.WithMaxRetries(1))

	_, err := augur.Query[actorFamily](context.Background(), client, &augur.Request{
		Query: "Tom Hanks family",
	})
	if !errors.Is(err, augur.ErrProviderFailure) {
		t.Fatalf("expected ErrProviderFailure, got %v", err)
	}
}

func TestQuery_ResponseHelpers(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		resp := envelope(`{"spouse":"Rita Wilson","children":["Colin"],"parents":["Amos"]}`)
		client := augur.New(newMock(resp))
		result, err := augur.Query[actorFamily](context.Background(), client, &augur.Request{
			Query: "Tom Hanks family",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.OK() {
			t.Error("expected OK() to be true for full success")
		}
		if result.IsPartial() {
			t.Error("expected IsPartial() to be false for full success")
		}
	})

	t.Run("Partial", func(t *testing.T) {
		resp := envelope(`{"spouse":"Rita Wilson","children":["Colin"]}`)
		client := augur.New(newMock(resp))
		result, err := augur.Query[actorFamily](context.Background(), client, &augur.Request{
			Query: "Tom Hanks family",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.OK() {
			t.Error("expected OK() to be false for partial success")
		}
		if !result.IsPartial() {
			t.Error("expected IsPartial() to be true for partial success")
		}
	})

	t.Run("TotalFailure", func(t *testing.T) {
		resp := envelope(`{"children":["Colin"]}`)
		client := augur.New(newMock(resp), augur.WithMaxRetries(0))
		result, err := augur.Query[actorFamily](context.Background(), client, &augur.Request{
			Query: "Tom Hanks family",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.OK() {
			t.Error("expected OK() to be false for total failure")
		}
		if result.IsPartial() {
			t.Error("expected IsPartial() to be false for total failure")
		}
		if result.Data != nil {
			t.Error("expected Data to be nil for total failure")
		}
	})
}

func TestSchemaFromFile_Nonexistent(t *testing.T) {
	_, err := augur.SchemaFromFile("/nonexistent/path/schema.json")
	if !errors.Is(err, augur.ErrSchemaInvalid) {
		t.Fatalf("expected ErrSchemaInvalid, got %v", err)
	}
}
