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

| Channel        | When                    | Meaning                                                              |
| -------------- | ----------------------- | -------------------------------------------------------------------- |
| `error` return | infrastructure failures | `ErrProviderFailure`, `ErrResponseMalformed`, `ErrSchemaInvalid`     |
| `resp.Data`    | data-level outcomes     | `nil` = total failure; non-nil = full or partial success             |
| `resp.Errors`  | field-level failures    | optional field absent or uncoercible; `resp.Data` is still usable    |

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

Per-query overrides via `Request.Options`:

```go
req := &augur.Request{
    Query: "...",
    Options: &augur.QueryOptions{
        Model:     "claude-haiku-4-5-20251001",
        MaxTokens: &maxTok,
    },
}
```

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
