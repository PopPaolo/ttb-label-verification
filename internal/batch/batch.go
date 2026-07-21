// Package batch implements asynchronous batch verification (DECISIONS.md D7):
// a zip + manifest.json upload (D6a) fans out to an in-process worker pool,
// with all state held in memory under TTL eviction (D8).
package batch

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"sync"
	"time"

	"ttb-label-verification/internal/extraction"
	"ttb-label-verification/internal/verify"
)

const (
	MaxApplications  = 500
	maxImagesPerApp  = 6
	maxImageBytes    = 8 << 20
	manifestFileName = "manifest.json"
)

// ManifestApp is one entry in manifest.json: the same application field set as
// POST /api/verify plus the two batch-only fields (D6a).
type ManifestApp struct {
	verify.Application
	ID     string   `json:"id"`
	Images []string `json:"images"`
}

type manifest struct {
	Applications []ManifestApp `json:"applications"`
}

// Item statuses. Terminal report statuses (pass/needs_review/fail) live in
// ReportStatus once Status is "done".
const (
	ItemQueued     = "queued"
	ItemProcessing = "processing"
	ItemDone       = "done"
	ItemFailed     = "failed"
)

type Item struct {
	ID           string             `json:"id"`
	Status       string             `json:"status"`
	ReportStatus string             `json:"report_status,omitempty"`
	Error        string             `json:"error,omitempty"`
	Report       *verify.Report     `json:"report,omitempty"`
	Extraction   *extraction.Result `json:"extraction,omitempty"`

	app    verify.Application
	images []extraction.Image // released after processing (D8)
}

type Batch struct {
	ID        string
	CreatedAt time.Time

	mu    sync.Mutex
	items []*Item
	index map[string]*Item
}

type job struct {
	batch *Batch
	item  *Item
}

// Summary is the triage view for GET /api/batches/{id} (SPEC §6: agents work
// the problem labels first).
type Summary struct {
	ID        string         `json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	Total     int            `json:"total"`
	Pending   int            `json:"pending"`
	Counts    map[string]int `json:"counts"`
	Items     []ItemSummary  `json:"items"`
}

type ItemSummary struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	ReportStatus string `json:"report_status,omitempty"`
	Error        string `json:"error,omitempty"`
}

func (b *Batch) summary() Summary {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := Summary{ID: b.ID, CreatedAt: b.CreatedAt, Total: len(b.items), Counts: map[string]int{}, Items: make([]ItemSummary, 0, len(b.items))}
	for _, it := range b.items {
		key := it.Status
		if it.Status == ItemDone {
			key = it.ReportStatus
		}
		s.Counts[key]++
		if it.Status == ItemQueued || it.Status == ItemProcessing {
			s.Pending++
		}
		s.Items = append(s.Items, ItemSummary{ID: it.ID, Status: it.Status, ReportStatus: it.ReportStatus, Error: it.Error})
	}
	return s
}

// Manager owns all in-flight batches and the worker pool.
type Manager struct {
	extractor extraction.Extractor
	logger    Logger
	ttl       time.Duration

	mu      sync.Mutex
	batches map[string]*Batch
	jobs    chan job
}

// Logger is the slice of *slog.Logger the manager needs (keeps tests simple).
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

func NewManager(extractor extraction.Extractor, logger Logger, workers int, ttl time.Duration) *Manager {
	if workers < 1 {
		workers = 1
	}
	m := &Manager{
		extractor: extractor,
		logger:    logger,
		ttl:       ttl,
		batches:   make(map[string]*Batch),
		jobs:      make(chan job, MaxApplications),
	}
	for i := 0; i < workers; i++ {
		go m.worker()
	}
	go m.janitor()
	return m
}

// Submit validates the archive and enqueues every application. Validation
// failures that poison the whole batch reject it up front (fail fast, D6a);
// per-application problems mark only that item failed.
func (m *Manager) Submit(zipData []byte) (*Batch, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("upload is not a valid zip archive")
	}

	files := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		files[path.Clean(f.Name)] = f
	}

	mf, ok := files[manifestFileName]
	if !ok {
		return nil, fmt.Errorf("manifest.json not found at the archive root")
	}
	raw, err := readZipFile(mf, 4<<20)
	if err != nil {
		return nil, fmt.Errorf("could not read manifest.json")
	}
	var man manifest
	if err := json.Unmarshal(raw, &man); err != nil {
		return nil, fmt.Errorf("manifest.json is not valid JSON: %v", err)
	}
	if len(man.Applications) == 0 {
		return nil, fmt.Errorf("manifest.json contains no applications")
	}
	if len(man.Applications) > MaxApplications {
		return nil, fmt.Errorf("batch exceeds the %d-application limit", MaxApplications)
	}
	seen := make(map[string]bool, len(man.Applications))
	for _, app := range man.Applications {
		if app.ID == "" {
			return nil, fmt.Errorf("every application needs a non-empty id")
		}
		if seen[app.ID] {
			return nil, fmt.Errorf("duplicate application id %q", app.ID)
		}
		seen[app.ID] = true
	}

	b := &Batch{ID: newID(), CreatedAt: time.Now(), index: make(map[string]*Item)}
	for _, app := range man.Applications {
		item := &Item{ID: app.ID, Status: ItemQueued, app: app.Application}
		if err := loadItemImages(item, app.Images, files); err != nil {
			item.Status = ItemFailed
			item.Error = err.Error()
		}
		b.items = append(b.items, item)
		b.index[app.ID] = item
	}

	m.mu.Lock()
	m.batches[b.ID] = b
	m.mu.Unlock()

	queued := 0
	for _, item := range b.items {
		if item.Status == ItemQueued {
			m.jobs <- job{batch: b, item: item}
			queued++
		}
	}
	m.logger.Info("batch submitted", "batch_id", b.ID, "applications", len(b.items), "queued", queued)
	return b, nil
}

func loadItemImages(item *Item, paths []string, files map[string]*zip.File) error {
	if len(paths) == 0 {
		return fmt.Errorf("no images listed for this application")
	}
	if len(paths) > maxImagesPerApp {
		return fmt.Errorf("more than %d images listed", maxImagesPerApp)
	}
	if err := validateApp(item.app); err != nil {
		return err
	}
	for _, p := range paths {
		f, ok := files[path.Clean(p)]
		if !ok {
			return fmt.Errorf("referenced image %q not found in archive", p)
		}
		data, err := readZipFile(f, maxImageBytes)
		if err != nil {
			return fmt.Errorf("could not read image %q: %v", p, err)
		}
		mediaType := http.DetectContentType(data)
		switch mediaType {
		case "image/jpeg", "image/png", "image/webp", "image/gif":
		default:
			return fmt.Errorf("%q is not a supported image type", p)
		}
		item.images = append(item.images, extraction.Image{Data: data, MediaType: mediaType})
	}
	return nil
}

func validateApp(app verify.Application) error {
	switch app.BeverageType {
	case verify.Wine, verify.Malt, verify.DistilledSpirits:
	default:
		return fmt.Errorf("beverage_type must be one of: wine, malt, distilled_spirits")
	}
	if app.BrandName == "" {
		return fmt.Errorf("brand_name is required")
	}
	return nil
}

func readZipFile(f *zip.File, limit int64) ([]byte, error) {
	if int64(f.UncompressedSize64) > limit {
		return nil, fmt.Errorf("file exceeds %d MB", limit>>20)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, limit))
}

func (m *Manager) Get(id string) (Summary, bool) {
	m.mu.Lock()
	b, ok := m.batches[id]
	m.mu.Unlock()
	if !ok {
		return Summary{}, false
	}
	return b.summary(), true
}

// GetItem returns a copy of one item's full report (same shape as /api/verify).
func (m *Manager) GetItem(batchID, itemID string) (Item, bool) {
	m.mu.Lock()
	b, ok := m.batches[batchID]
	m.mu.Unlock()
	if !ok {
		return Item{}, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	item, ok := b.index[itemID]
	if !ok {
		return Item{}, false
	}
	return *item, true
}

func (m *Manager) worker() {
	for j := range m.jobs {
		m.process(j)
	}
}

func (m *Manager) process(j job) {
	b, item := j.batch, j.item

	b.mu.Lock()
	images := item.images
	app := item.app
	item.Status = ItemProcessing
	b.mu.Unlock()

	start := time.Now()
	result, err := m.extractor.Extract(context.Background(), images)

	b.mu.Lock()
	defer b.mu.Unlock()
	item.images = nil // images are processed and discarded (D8)
	if err != nil {
		// Partial-failure semantics (D7): this item fails, the batch continues.
		item.Status = ItemFailed
		item.Error = "could not read the label image — request a better image or retry"
		m.logger.Error("batch item extraction failed", "batch_id", b.ID, "item_id", item.ID, "error", err)
		return
	}
	report := verify.Verify(app, result)
	item.Status = ItemDone
	item.ReportStatus = report.Status
	item.Report = &report
	item.Extraction = result
	m.logger.Info("batch item complete", "batch_id", b.ID, "item_id", item.ID, "status", report.Status, "ms", time.Since(start).Milliseconds())
}

func (m *Manager) janitor() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		cutoff := time.Now().Add(-m.ttl)
		m.mu.Lock()
		for id, b := range m.batches {
			if b.CreatedAt.Before(cutoff) {
				delete(m.batches, id)
			}
		}
		m.mu.Unlock()
	}
}

func newID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
