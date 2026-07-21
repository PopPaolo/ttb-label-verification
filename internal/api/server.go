// Package api wires the HTTP surface: JSON REST endpoints plus the static
// frontend assets, served from one binary (DECISIONS.md D1, D6).
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"ttb-label-verification/internal/batch"
	"ttb-label-verification/internal/extraction"
)

type Server struct {
	logger    *slog.Logger
	staticDir string
	extractor extraction.Extractor
	batches   *batch.Manager
	mux       *http.ServeMux
}

func NewServer(logger *slog.Logger, staticDir string, extractor extraction.Extractor, batches *batch.Manager) *Server {
	s := &Server{
		logger:    logger,
		staticDir: staticDir,
		extractor: extractor,
		batches:   batches,
		mux:       http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/verify", s.handleVerify)
	s.mux.HandleFunc("POST /api/batches", s.handleBatchSubmit)
	s.mux.HandleFunc("GET /api/batches/{id}", s.handleBatchStatus)
	s.mux.HandleFunc("GET /api/batches/{id}/items/{itemID}", s.handleBatchItem)
	s.mux.HandleFunc("/", s.handleStatic)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleStatic serves the built SPA. Unknown paths fall back to index.html so
// client-side routing works; missing build directory returns a plain notice
// (useful in dev when only the API is running).
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(s.staticDir, filepath.Clean("/"+r.URL.Path))
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		http.ServeFile(w, r, path)
		return
	}
	index := filepath.Join(s.staticDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		http.Error(w, "frontend not built; run `npm run build` in web/", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, index)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
