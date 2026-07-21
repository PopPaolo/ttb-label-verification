package main

import (
	"log/slog"
	"net/http"
	"os"

	"ttb-label-verification/internal/api"
	"ttb-label-verification/internal/batch"
	"ttb-label-verification/internal/config"
	"ttb-label-verification/internal/extraction"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	if cfg.AnthropicAPIKey == "" {
		logger.Warn("ANTHROPIC_API_KEY is not set; extraction calls will fail")
	}

	extractor := extraction.NewClaudeExtractor(cfg.AnthropicAPIKey, cfg.Model, cfg.ExtractionEffort, cfg.ExtractionTimeout)
	batches := batch.NewManager(extractor, logger, cfg.BatchWorkers, cfg.BatchTTL)
	srv := api.NewServer(logger, cfg.StaticDir, extractor, batches)
	logger.Info("starting server", "addr", cfg.Addr, "model", cfg.Model)
	if err := http.ListenAndServe(cfg.Addr, srv); err != nil {
		logger.Error("server exited", "error", err)
		os.Exit(1)
	}
}
