# augur rest-api example

A basic HTTP service that exposes Augur as a single `POST /query` endpoint for local testing and example SDK usage.

## Setup

```sh
cp .env.example .env
# Edit .env and set ANTHROPIC_API_KEY
```

## Running

**Standard**

```sh
go run .
```

**Hot reload with [Air](https://github.com/air-verse/air)**

```sh
# Install Air (one-time)
go install github.com/air-verse/air@latest

# Run with hot reload
make dev
```

The server listens on `:8080`.

## Environment variables

| Variable            | Required | Default             | Description                        |
| ------------------- | -------- | ------------------- | ---------------------------------- |
| `ANTHROPIC_API_KEY` | Yes      | —                   | Anthropic API key                  |
| `AUGUR_MODEL`       | No       | `claude-sonnet-4-6` | Model identifier                   |
| `AUGUR_MAX_TOKENS`  | No       | `8192`              | Max output tokens                  |
| `AUGUR_MAX_RETRIES` | No       | `2`                 | Retry attempts for required fields |

## SDK Usage

A basic example of how to take input and make a call using the Augur client.

```go
// Build schema from the raw JSON object in the request body.
schemaBytes, _ := json.Marshal(req.Schema)
schema, err := augur.SchemaFromJSON(string(schemaBytes))
if err != nil {
  writeError(w, http.StatusBadRequest, "invalid schema: "+err.Error())
  return
}

augurReq := &augur.Request{
  Query:   req.Query,
  Schema:  schema,
  Context: req.Context,
}
if req.Options != nil {
  augurReq.Options = &augur.QueryOptions{
    Model:       req.Options.Model,
    Temperature: req.Options.Temperature,
    MaxTokens:   req.Options.MaxTokens,
  }
  // Enable source citations backed by web search.
  if req.Options.Sources != nil {
    augurReq.Options.Sources = &augur.SourceConfig{
      MaxSearches:    req.Options.Sources.MaxSearches,
      AllowedDomains: req.Options.Sources.AllowedDomains,
      BlockedDomains: req.Options.Sources.BlockedDomains,
    }
  }
}

// Use map[string]any since the schema is dynamic at the HTTP layer.
resp, err := augur.Query[map[string]any](r.Context(), client, augurReq)
```

### Error Handling

A basic example of checking for infrastructure failure, total failure, and returning success responses.

```go
// Infrastructure failure — no usable response.
if err != nil {
  logger.Error("query failed", "error", err, "query", req.Query)
  writeError(w, http.StatusInternalServerError, "internal error")
  return
}

// Total failure — required fields unresolvable after retries.
if resp.Data == nil {
  w.WriteHeader(http.StatusUnprocessableEntity)
  json.NewEncoder(w).Encode(resp)
  return
}

// Full or partial success — data is usable; caller inspects resp.Errors.
w.WriteHeader(http.StatusOK)
json.NewEncoder(w).Encode(resp)
```

## API

### `POST /query`

**Request body**

```json
{
  "query": "Natural language description of the data to retrieve",
  "schema": {
    "type": "object",
    "properties": {
      "field_name": { "type": "string", "description": "What this field is" }
    },
    "required": ["field_name"]
  },
  "context": "Optional additional context for the LLM",
  "options": {
    "model": "claude-sonnet-4-6",
    "maxTokens": 4096,
    "sources": {
      "maxSearches": 5,
      "allowedDomains": ["wikipedia.org"],
      "blockedDomains": []
    }
  }
}
```

**Response — full success (`200`)**

```json
{
  "data": { "field_name": "value" },
  "meta": {
    "field_name": {
      "confidence": 0.95,
      "sources": [{ "url": "...", "title": "...", "citedText": "..." }]
    }
  },
  "errors": [],
  "notes": "",
  "provider": "claude",
  "model": "claude-sonnet-4-6",
  "retriesExecuted": 0,
  "latencyMs": 1234,
  "usage": { "inputTokens": 512, "outputTokens": 128, "webSearchRequests": 2 }
}
```

> **Note:** `sources` are only populated when `options.sources` is provided in the request.
> Without it, sources will always be empty arrays. Enabling sources activates web search,
> which incurs additional cost ($10 per 1,000 searches) and increases token usage.
> The `webSearchRequests` field in `usage` is only present when sources are enabled.
```

**Response — partial success (`200`)**

Same shape as full success, but `errors` contains entries for optional fields that could not be resolved. `data` is still usable.

**Response — total failure (`422`)**

Required fields could not be resolved after retries. `data` is `null`; `errors` describes which fields failed.

```json
{
  "data": null,
  "errors": [{ "field": "field_name", "reason": "field not present in response" }],
  ...
}
```

**Response — bad request (`400`)**

```json
{ "error": "query is required" }
```

**Response — server error (`500`)**

```json
{ "error": "internal error" }
```

## Example requests

**Net worth lookup**

```sh
curl -s -X POST http://localhost:8080/query \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "Tom Hanks net worth",
    "schema": {
      "type": "object",
      "properties": {
        "amount":   { "type": "integer", "description": "Net worth in USD" },
        "currency": { "type": "string",  "description": "Currency code" }
      },
      "required": ["amount"]
    }
  }' | jq .
```

**Actor biography**

```sh
curl -s -X POST http://localhost:8080/query \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "Tom Hanks biography",
    "context": "Focus on personal life",
    "schema": {
      "type": "object",
      "properties": {
        "spouse":   { "type": "string",  "description": "Current or most recent spouse" },
        "children": { "type": "array",   "items": { "type": "string" }, "description": "Names of children" },
        "born":     { "type": "integer", "description": "Birth year" }
      },
      "required": ["spouse", "born"]
    }
  }' | jq .
```

**Per-query model override**

```sh
curl -s -X POST http://localhost:8080/query \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "Current CEO of Apple",
    "schema": {
      "type": "object",
      "properties": {
        "name":  { "type": "string" },
        "since": { "type": "integer", "description": "Year they became CEO" }
      },
      "required": ["name"]
    },
    "options": { "model": "claude-haiku-4-5-20251001" }
  }' | jq .
```

**With source citations (web search enabled)**

```sh
curl -s -X POST http://localhost:8080/query \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "Tom Hanks biography",
    "context": "Focus on personal life",
    "schema": {
      "type": "object",
      "properties": {
        "spouse":   { "type": "string",  "description": "Current or most recent spouse" },
        "children": { "type": "array",   "items": { "type": "string" }, "description": "Names of children" },
        "born":     { "type": "integer", "description": "Birth year" }
      },
      "required": ["spouse", "born"]
    },
    "options": {
      "sources": {
        "maxSearches": 3,
        "allowedDomains": ["wikipedia.org", "britannica.com"]
      }
    }
  }' | jq .
```
