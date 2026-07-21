package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"ttb-label-verification/internal/extraction"
	"ttb-label-verification/internal/verify"
)

const (
	maxUploadBytes = 32 << 20 // whole multipart form
	maxImageBytes  = 8 << 20  // per label piece
	maxImages      = 6
)

var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

// VerifyResponse is the D6 single-verification report: deterministic rule
// results plus the raw extraction (for the "where was this found" UI) and
// per-stage timings (D13 latency SLO).
type VerifyResponse struct {
	Status     string              `json:"status"`
	Results    []verify.RuleResult `json:"results"`
	Extraction *extraction.Result  `json:"extraction"`
	TimingsMS  map[string]int64    `json:"timings_ms"`
}

type apiError struct {
	Error string `json:"error"`
}

// handleVerify implements POST /api/verify: multipart form with an
// `application` JSON field and one or more `images` files. Synchronous — the
// 5-second budget makes polling ceremony pointless (D6).
func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{"could not read upload — send a multipart form with an 'application' field and 'images' files"})
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	app, err := parseApplication(r.FormValue("application"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{err.Error()})
		return
	}

	images, err := readImages(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{err.Error()})
		return
	}

	extractStart := time.Now()
	result, err := s.extractor.Extract(r.Context(), images)
	extractDur := time.Since(extractStart)
	if err != nil {
		s.logger.Error("extraction failed", "error", err)
		writeJSON(w, http.StatusBadGateway, apiError{"could not read the label image — try again, or upload a clearer image"})
		return
	}

	verifyStart := time.Now()
	report := verify.Verify(*app, result)
	verifyDur := time.Since(verifyStart)

	s.logger.Info("verify complete",
		"status", report.Status,
		"beverage_type", app.BeverageType,
		"images", len(images),
		"extraction_ms", extractDur.Milliseconds(),
		"verification_ms", verifyDur.Milliseconds(),
		"total_ms", time.Since(start).Milliseconds(),
	)

	writeJSON(w, http.StatusOK, VerifyResponse{
		Status:     report.Status,
		Results:    report.Results,
		Extraction: result,
		TimingsMS: map[string]int64{
			"extraction":   extractDur.Milliseconds(),
			"verification": verifyDur.Milliseconds(),
			"total":        time.Since(start).Milliseconds(),
		},
	})
}

func parseApplication(raw string) (*verify.Application, error) {
	if raw == "" {
		return nil, fmt.Errorf("missing 'application' field — include the application data as JSON")
	}
	var app verify.Application
	if err := json.Unmarshal([]byte(raw), &app); err != nil {
		return nil, fmt.Errorf("application data is not valid JSON: %v", err)
	}
	switch app.BeverageType {
	case verify.Wine, verify.Malt, verify.DistilledSpirits:
	default:
		return nil, fmt.Errorf("beverage_type must be one of: wine, malt, distilled_spirits")
	}
	if app.BrandName == "" {
		return nil, fmt.Errorf("brand_name is required")
	}
	return &app, nil
}

func readImages(r *http.Request) ([]extraction.Image, error) {
	files := r.MultipartForm.File["images"]
	if len(files) == 0 {
		return nil, fmt.Errorf("upload at least one label image in the 'images' field")
	}
	if len(files) > maxImages {
		return nil, fmt.Errorf("at most %d label images per application", maxImages)
	}

	images := make([]extraction.Image, 0, len(files))
	for _, fh := range files {
		if fh.Size > maxImageBytes {
			return nil, fmt.Errorf("image %q is too large (max %d MB)", fh.Filename, maxImageBytes>>20)
		}
		f, err := fh.Open()
		if err != nil {
			return nil, fmt.Errorf("could not read image %q", fh.Filename)
		}
		data := make([]byte, fh.Size)
		if _, err := io.ReadFull(f, data); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("could not read image %q", fh.Filename)
		}
		_ = f.Close()

		mediaType := http.DetectContentType(data)
		if !allowedImageTypes[mediaType] {
			return nil, fmt.Errorf("%q is not a supported image type (use JPEG, PNG, WebP, or GIF)", fh.Filename)
		}
		images = append(images, extraction.Image{Data: data, MediaType: mediaType})
	}
	return images, nil
}
