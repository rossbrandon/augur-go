package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	augur "github.com/rossbrandon/augur-go"
	"github.com/rossbrandon/augur-go/providers/claude"
)

func main() {
	// Load .env if present — silently ignored if the file doesn't exist.
	_ = godotenv.Load()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	opts := []augur.Option{
		augur.WithLogger(logger),
	}
	if model := os.Getenv("AUGUR_MODEL"); model != "" {
		opts = append(opts, augur.WithModel(model))
	}
	if v := os.Getenv("AUGUR_MAX_TOKENS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			opts = append(opts, augur.WithMaxTokens(n))
		}
	}
	if v := os.Getenv("AUGUR_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			opts = append(opts, augur.WithMaxRetries(n))
		}
	}
	if v := os.Getenv("AUGUR_DISABLE_WEB_SEARCH"); v != "" {
		if v == "true" {
			opts = append(opts, augur.WithoutWebSearch())
		}
	}

	client := augur.New(
		claude.NewProvider(os.Getenv("ANTHROPIC_API_KEY")),
		opts...,
	)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(120 * time.Second))

	r.Post("/query", handleQuery(client, logger))

	addr := ":8080"
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
