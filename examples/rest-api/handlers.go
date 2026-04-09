package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	augur "github.com/rossbrandon/augur-go"
)

// queryRequest is the HTTP request body for POST /query.
type queryRequest struct {
	Query   string         `json:"query"`
	Schema  map[string]any `json:"schema"`
	Context string         `json:"context,omitempty"`
	Options *queryOptions  `json:"options,omitempty"`
}

// queryOptions mirrors augur.QueryOptions for HTTP deserialization.
type queryOptions struct {
	Model       string        `json:"model,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"maxTokens,omitempty"`
	Sources     *sourceConfig `json:"sources,omitempty"`
}

// sourceConfig mirrors augur.SourceConfig for HTTP deserialization.
type sourceConfig struct {
	Disabled       bool     `json:"disabled,omitempty"`
	MaxSearches    *int     `json:"maxSearches,omitempty"`
	AllowedDomains []string `json:"allowedDomains,omitempty"`
	BlockedDomains []string `json:"blockedDomains,omitempty"`
}

func handleQuery(client *augur.Client, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req queryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Query == "" {
			writeError(w, http.StatusBadRequest, "query is required")
			return
		}
		if req.Schema == nil {
			writeError(w, http.StatusBadRequest, "schema is required")
			return
		}

		schemaBytes, err := json.Marshal(req.Schema)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid schema")
			return
		}
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
			if req.Options.Sources != nil {
				augurReq.Options.Sources = &augur.SourceConfig{
					Disabled:       req.Options.Sources.Disabled,
					MaxSearches:    req.Options.Sources.MaxSearches,
					AllowedDomains: req.Options.Sources.AllowedDomains,
					BlockedDomains: req.Options.Sources.BlockedDomains,
				}
			}
		}

		resp, err := augur.Query[map[string]any](r.Context(), client, augurReq)
		if err != nil {
			if errors.Is(err, augur.ErrSourcesNotSupported) {
				writeError(w, http.StatusBadRequest, "sources are not supported by the requested model")
				return
			}
			logger.Error("query failed", "error", err, "query", req.Query)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if resp.Data == nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(resp)
			return
		}

		if resp.IsPartial() {
			logger.Warn("partial result", "query", req.Query, "errors", resp.Errors)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
