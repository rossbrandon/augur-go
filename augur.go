package augur

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Query executes a structured data retrieval against the configured LLM provider.
//
// T is the caller's output type and serves as the schema definition when
// req.Schema is nil. When req.Schema is provided explicitly (via SchemaFromJSON
// or SchemaFromFile), it takes precedence over reflection.
//
// Error handling follows two channels:
//   - Returned error: infrastructure failures only (network, auth, malformed
//     schema, unparseable JSON). When err is non-nil, resp is nil.
//   - resp (always non-nil when err is nil): data-level outcomes.
//     resp.Data == nil means total failure (required fields unresolved after
//     retries); check resp.Errors. resp.Data != nil means full or partial
//     success; check resp.Errors for any optional fields that could not be
//     populated.
func Query[T any](ctx context.Context, c *Client, req *Request) (*Response[T], error) {
	start := time.Now()

	if req == nil {
		return nil, fmt.Errorf("%w: request must not be nil", ErrInvalidRequest)
	}
	if req.Query == "" {
		return nil, fmt.Errorf("%w: query must not be empty", ErrInvalidRequest)
	}

	schema, err := resolveSchema[T](req)
	if err != nil {
		return nil, err
	}

	model := c.model
	temperature := 0.0
	maxTokens := c.maxTokens
	if req.Options != nil {
		if req.Options.Model != "" {
			model = req.Options.Model
		}
		if req.Options.Temperature != nil {
			temperature = *req.Options.Temperature
		}
		if req.Options.MaxTokens != nil {
			maxTokens = *req.Options.MaxTokens
		}
	}

	userPrompt, err := buildUserPrompt(req, schema)
	if err != nil {
		return nil, fmt.Errorf("building prompt: %w", err)
	}

	if c.logger != nil {
		c.logger.Debug("executing query", "model", model, "query", req.Query)
	}

	// Execute provider call with retry loop for required field failures.
	params := &ProviderParams{
		SystemPrompt: systemPromptTemplate,
		UserPrompt:   userPrompt,
		Model:        model,
		Temperature:  temperature,
		MaxTokens:    maxTokens,
	}

	result, providerModel, usage, retries, err := executeWithRetry(ctx, c, req, schema, params)
	if err != nil {
		return nil, err
	}

	latency := time.Since(start).Milliseconds()

	resp := &Response[T]{
		Meta:            buildResponseMeta(result.meta),
		Errors:          result.errors,
		Notes:           result.notes,
		Provider:        c.provider.Name(),
		Model:           providerModel,
		RetriesExecuted: retries,
		LatencyMS:       latency,
		Usage:           usage,
	}

	// Total failure: required fields remain unresolved after all retries.
	// Return resp with Data=nil so the caller can inspect resp.Errors.
	failedRequired := requiredFailures(result, schema)
	if len(failedRequired) > 0 {
		if c.logger != nil {
			c.logger.Debug("total failure: required fields unresolved", "fields", failedRequired)
		}
		return resp, nil
	}

	// Unmarshal coerced data map into T.
	dataBytes, err := json.Marshal(result.data)
	if err != nil {
		return nil, fmt.Errorf("marshaling result data: %w", err)
	}
	var typed T
	if err := json.Unmarshal(dataBytes, &typed); err != nil {
		return nil, fmt.Errorf("unmarshaling result into %T: %w", typed, err)
	}
	resp.Data = &typed

	return resp, nil
}

// executeWithRetry runs the provider and validation pipeline, retrying up to
// maxRetries times for required field failures. Returns the final processResult,
// the actual model name, and token usage.
func executeWithRetry(
	ctx context.Context,
	c *Client,
	req *Request,
	schema *Schema,
	params *ProviderParams,
) (*processResult, string, *Usage, int, error) {
	var result *processResult
	var providerModel string
	var usage *Usage
	var retries int
	currentParams := params

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			retries = attempt
			if err := ctx.Err(); err != nil {
				return nil, providerModel, usage, retries, err
			}
			if c.logger != nil {
				c.logger.Debug("retrying required fields", "attempt", attempt, "max", c.maxRetries)
			}
		}

		providerResult, err := c.provider.Execute(ctx, currentParams)
		if err != nil {
			return nil, "", nil, retries, fmt.Errorf("%w: %w", ErrProviderFailure, err)
		}

		providerModel = providerResult.Model
		if providerResult.Usage != nil {
			if usage == nil {
				usage = &Usage{}
			}
			usage.InputTokens += providerResult.Usage.InputTokens
			usage.OutputTokens += providerResult.Usage.OutputTokens
		}

		pr, err := processResponse(providerResult.Content, schema, c.logger)
		if err != nil {
			return nil, providerModel, usage, retries, err
		}

		if attempt == 0 {
			result = pr
		} else {
			mergeRetryResult(result, pr)
		}

		// Check if any required fields still need resolution.
		failed := requiredFailures(result, schema)
		if len(failed) == 0 {
			break
		}

		if attempt < c.maxRetries {
			retryPrompt := buildRetryPrompt(req, failed, schema)
			currentParams = &ProviderParams{
				SystemPrompt: systemPromptTemplate,
				UserPrompt:   retryPrompt,
				Model:        params.Model,
				Temperature:  params.Temperature,
				MaxTokens:    params.MaxTokens,
			}
		}
	}

	return result, providerModel, usage, retries, nil
}

// resolveSchema returns the schema to use for the query. If req.Schema is set,
// it takes precedence. Otherwise the schema is derived from T via reflection.
// Returns ErrSchemaInvalid if neither path yields a valid schema.
func resolveSchema[T any](req *Request) (*Schema, error) {
	if req.Schema != nil {
		return req.Schema, nil
	}

	schema, err := SchemaFromType[T]()
	if err != nil {
		return nil, err
	}

	// Guard against struct types with no exported fields, which produce a valid
	// but empty schema that would yield no useful output.
	if len(schema.properties) == 0 {
		return nil, fmt.Errorf("%w: type has no exported fields; provide a schema explicitly via SchemaFromJSON or SchemaFromFile", ErrSchemaInvalid)
	}

	return schema, nil
}
