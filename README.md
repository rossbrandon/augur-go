# augur-go

Augur is a model-agnostic Go library that uses LLMs as a structured, schema-aware data retrieval layer. You describe what you want in natural language, define the output shape with a Go struct or JSON Schema, and get back strongly-typed, validated data with per-field source attribution.

## Installation

```sh
go get github.com/rossbrandon/augur-go
```

Requires Go 1.23+.

## Quick start

```go
import (
    augur "github.com/rossbrandon/augur-go"
    "github.com/rossbrandon/augur-go/providers/claude"
)

// 1. Define the output shape with struct tags.
type ActorInfo struct {
    NetWorth int64    `json:"netWorth" augur:"required,desc:Estimated net worth"`
    Currency string   `json:"currency" augur:"default:USD,desc:Currency code of net worth"`
    Spouse   string   `json:"spouse"   augur:"required,desc:Current or most recent spouse"`
    Children []string `json:"children" augur:"required,desc:Names of children"`
    AsOfYear int32    `json:"asOfYear" augur:"desc:Estimated year of data"`
}

// 2. Create a client.
client := augur.New(claude.NewProvider(os.Getenv("ANTHROPIC_API_KEY")))

// 3. Query.
resp, err := augur.Query[ActorInfo](ctx, client, &augur.Request{
    Query:   "Tom Hanks net worth and family relationships",
    Context: "Focus on USD financials and immediate family",
})
```

## Data flow

```
Caller
  │  Query[T] + Request{Query, Schema, Context}
  ▼
augur.Client
  │  builds system + user prompts
  ▼
Provider (claude / openai / …)
  │  raw LLM text
  ▼
validate pipeline
  ├─ JSON extraction (direct → markdown fence strip → brace scan)
  ├─ envelope parse  {"data":{…}, "meta":{…}, "notes":"…"}
  ├─ type coercion   string→int, scalar→array, "400M"→400000000, …
  └─ required-field check → retry loop (up to maxRetries)
  ▼
Response[T]{Data, Meta, Errors, Notes, Usage, RetriesExecuted, LatencyMS}
```

## Error handling

Augur uses two error channels:

| Channel        | When                    | Meaning                                                           |
| -------------- | ----------------------- | ----------------------------------------------------------------- |
| `error` return | infrastructure failures | `ErrProviderFailure`, `ErrResponseMalformed`, `ErrSchemaInvalid`  |
| `resp.Data`    | data-level outcomes     | `nil` = total failure; non-nil = full or partial success          |
| `resp.Errors`  | field-level failures    | optional field absent or uncoercible; `resp.Data` is still usable |

When `err` is non-nil, `resp` is nil — something broke at the infrastructure level (network, auth, malformed JSON).

When `err` is nil, `resp` is always non-nil. Check `resp.Data`:

- **`resp.Data == nil`** (total failure): required fields could not be resolved after retries. Check `resp.Errors`.
- **`resp.Data != nil`** with `resp.Errors`: partial success — some optional fields missing.
- **`resp.Data != nil`** with no errors: full success.

```go
resp, err := augur.Query[ActorInfo](ctx, client, req)
if err != nil {
    return fmt.Errorf("augur: %w", err) // infrastructure failure
}

if resp.Data == nil {
    // Total failure — required fields unresolved.
    log.Printf("failed fields: %v", resp.Errors)
    return nil
}

if resp.IsPartial() {
    // Partial success — some optional fields missing.
    log.Printf("partial result, missing: %v", resp.Errors)
}

// Full or partial success — data is usable.
fmt.Printf("net worth: %d %s\n", resp.Data.NetWorth, resp.Data.Currency)
```

## Schema definition

**Struct tags (recommended)**

The `ActorInfo` struct above demonstrates all three directives. To build a schema explicitly:

```go
schema, err := augur.SchemaFromType[ActorInfo]()
```

Supported `augur` tag directives:

| Directive     | Effect                                                                              |
| ------------- | ----------------------------------------------------------------------------------- |
| `required`    | Field must be present; triggers retry if absent                                     |
| `desc:text`   | Adds description to the schema (improves LLM accuracy); commas in the text are safe |
| `default:val` | Applied when field is absent from the LLM response                                  |

**JSON Schema string**

```go
schema, err := augur.SchemaFromJSON(`{
    "type": "object",
    "properties": {
      "netWorth": {
        "type": "integer",
        "description": "Estimated net worth"
      },
      "currency": {
        "type": "string",
        "description": "Currency code of net worth"
      },
      "spouse": {
        "type": "string",
        "description": "Name of current or most recent spouse"
      },
      "children": {
        "type": "array",
        "items": {
          "type": "string"
        },
        "description": "Biological and adopted children"
      },
      "asOfYear": {
        "type": "integer",
        "description": "Estimated year of retrieved data"
      }
    },
    "required": [
      "netWorth",
      "spouse",
      "children"
    ]
}`)
```

**File**

```go
schema, err := augur.SchemaFromFile("schema/actor.json")
```

When `req.Schema` is set it always takes precedence over `T`.

## Response metadata

Every field in `resp.Meta` includes a confidence score. Source citations are populated by default via [web search](#source-citations).

```go
if meta, ok := resp.Meta["netWorth"]; ok {
    fmt.Printf("confidence: %.2f\n", meta.Confidence)
    for _, src := range meta.Sources {
        fmt.Printf("  source: %s — %s\n", src.Title, src.URL)
    }
}
fmt.Printf("model: %s, latency: %dms, tokens: %d in / %d out\n",
    resp.Model, resp.LatencyMS,
    resp.Usage.InputTokens, resp.Usage.OutputTokens)
```

## Client options

```go
client := augur.New(
    claude.NewProvider(apiKey),
    augur.WithModel("claude-sonnet-4-6"),
    augur.WithMaxTokens(4096),
    augur.WithMaxRetries(2),
    augur.WithLogger(slog.Default()),
)
```

Web search is enabled by default. To disable it globally or customize the default configuration:

```go
// Disable web search for all queries.
client := augur.New(provider, augur.WithoutWebSearch())

// Customize default web search settings for all queries.
client := augur.New(provider, augur.WithSourceConfig(augur.SourceConfig{
    MaxSearches:    augur.Int(5),
    AllowedDomains: []string{"wikipedia.org", "britannica.com"},
}))
```

Per-query overrides via `Request.Options`:

```go
req := &augur.Request{
    Query: "...",
    Options: &augur.QueryOptions{
        Model:     "claude-haiku-4-5-20251001",
        MaxTokens: augur.Int(4096),
    },
}
```

## Source citations

Web search is **enabled by default**. Every query automatically uses web search to ground the model in real-time data, and `FieldMeta.Sources` will contain real, verifiable URLs from search results.

```go
resp, err := augur.Query[ActorInfo](ctx, client, &augur.Request{
    Query: "Harrison Ford net worth and family",
})

// Sources contain real URLs from web search results.
for _, src := range resp.Meta["netWorth"].Sources {
    fmt.Printf("  %s — %s\n", src.Title, src.URL)
    // e.g. "Harrison Ford Net Worth | Celebrity Net Worth" — https://www.celebritynetworth.com/...
}
```

### Disabling web search

To disable web search for a specific query, set `Disabled: true` on `SourceConfig`:

```go
resp, err := augur.Query[ActorInfo](ctx, client, &augur.Request{
    Query: "...",
    Options: &augur.QueryOptions{
        Sources: &augur.SourceConfig{Disabled: true},
    },
})
```

To disable web search globally, use `WithoutWebSearch()` when creating the client:

```go
client := augur.New(provider, augur.WithoutWebSearch())
```

### SourceConfig options

| Field            | Type       | Default | Description                                                                 |
| ---------------- | ---------- | ------- | --------------------------------------------------------------------------- |
| `Disabled`       | `bool`     | `false` | Turns off web search entirely when true.                                    |
| `MaxSearches`    | `*int`     | `2`     | Max web searches per query. Higher values find more data but increase cost. |
| `AllowedDomains` | `[]string` | all     | Restrict results to these domains only.                                     |
| `BlockedDomains` | `[]string` | none    | Exclude results from these domains.                                         |

```go
Sources: &augur.SourceConfig{
    MaxSearches:    augur.Int(3),
    AllowedDomains: []string{"wikipedia.org", "britannica.com"},
}
```

### Cost and performance

Web search adds cost and latency to every query (since it is on by default):

- **Per-search fee**: $10 per 1,000 searches (each search counts as one use regardless of results returned).
- **Token usage**: Search result content is loaded into the context window as input tokens.
- **Latency**: Each search adds network round-trip time.

For cost-sensitive or high-volume workloads, disable web search at the client level with `augur.WithoutWebSearch()` or per-query with `&augur.SourceConfig{Disabled: true}`.

The `Usage` field on the response tracks web search usage alongside token consumption:

```go
fmt.Printf("searches: %d, tokens: %d in / %d out\n",
    resp.Usage.WebSearchRequests,
    resp.Usage.InputTokens,
    resp.Usage.OutputTokens)
```

### Model compatibility

Not all models support web search. If the requested model is incompatible, `Query` returns `ErrSourcesNotSupported`:

```go
resp, err := augur.Query[ActorInfo](ctx, client, req)
if errors.Is(err, augur.ErrSourcesNotSupported) {
    // switch to a compatible model or disable sources
}
```

### When to disable web search

| Scenario                                             | Web search | Why                                      |
| ---------------------------------------------------- | ---------- | ---------------------------------------- |
| Factual data that needs verifiable citations         | On         | Real URLs from web search (default)      |
| Current/real-time data (prices, events, recent news) | On         | Web search bypasses training data cutoff |
| Cost-sensitive, training data is sufficient          | Off        | Avoids web search cost and latency       |
| High-volume batch queries                            | Off        | Cost scales with search count            |

## Type coercion

Augur automatically coerces common LLM output patterns to the declared schema type:

| Input                      | Target type | Result            |
| -------------------------- | ----------- | ----------------- |
| `"$400,000,000"`           | integer     | `400000000`       |
| `"1.5 billion"`            | integer     | `1500000000`      |
| `"400M"`                   | number      | `400000000.0`     |
| `"true"` / `"yes"` / `"1"` | boolean     | `true`            |
| `"Rita Wilson"`            | `[]string`  | `["Rita Wilson"]` |

## Provider implementations

| Package                                            | Provider         |
| -------------------------------------------------- | ---------------- |
| `github.com/rossbrandon/augur-go/providers/claude` | Anthropic Claude |

Implement the `augur.Provider` interface to add a new backend — only two methods required: `Execute` and `Name`.

## Examples

- [`examples/rest-api`](examples/rest-api) — HTTP service that exposes Augur as a REST endpoint
