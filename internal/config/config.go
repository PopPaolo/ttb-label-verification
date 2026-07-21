// Package config loads runtime configuration from environment variables (DECISIONS.md D11).
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Addr is the listen address for the HTTP server, e.g. ":8080".
	Addr string
	// AnthropicAPIKey authenticates extraction calls to the Anthropic API (D4).
	AnthropicAPIKey string
	// Model is the vision model used for label extraction.
	Model string
	// ExtractionEffort is the output_config effort level for extraction calls (D4/D13).
	ExtractionEffort string
	// BatchWorkers bounds the batch worker pool (D7).
	BatchWorkers int
	// ExtractionTimeout caps a single extraction call (D13).
	ExtractionTimeout time.Duration
	// StaticDir is the directory of built frontend assets served by the binary (D1/D3).
	StaticDir string
}

func Load() Config {
	return Config{
		Addr:              getenv("ADDR", ":8080"),
		AnthropicAPIKey:   os.Getenv("ANTHROPIC_API_KEY"),
		Model:             getenv("EXTRACTION_MODEL", "claude-opus-4-8"),
		ExtractionEffort:  getenv("EXTRACTION_EFFORT", "low"),
		BatchWorkers:      getenvInt("BATCH_WORKERS", 8),
		ExtractionTimeout: getenvDuration("EXTRACTION_TIMEOUT", 30*time.Second),
		StaticDir:         getenv("STATIC_DIR", "web/dist"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
