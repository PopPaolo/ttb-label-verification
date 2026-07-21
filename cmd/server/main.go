package main

import (
	"log/slog"
	"net/http"
	"os"

	"ttb-label-verification/internal/api"
	"ttb-label-verification/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	if cfg.AnthropicAPIKey == "" {
		logger.Warn("ANTHROPIC_API_KEY is not set; extraction calls will fail")
	}

	srv := api.NewServer(logger, cfg.StaticDir)
	logger.Info("starting server", "addr", cfg.Addr, "model", cfg.Model)
	if err := http.ListenAndServe(cfg.Addr, srv); err != nil {
		logger.Error("server exited", "error", err)
		os.Exit(1)
	}
}
