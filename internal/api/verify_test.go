package api

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ttb-label-verification/internal/batch"
	"ttb-label-verification/internal/extraction"
	"ttb-label-verification/internal/verify"
)

type stubExtractor struct {
	result *extraction.Result
	err    error
}

func (s *stubExtractor) Extract(_ context.Context, _ []extraction.Image) (*extraction.Result, error) {
	return s.result, s.err
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func multipartRequest(t *testing.T, appJSON string, images ...[]byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if appJSON != "" {
		if err := w.WriteField("application", appJSON); err != nil {
			t.Fatal(err)
		}
	}
	for _, img := range images {
		fw, err := w.CreateFormFile("images", "label.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(img); err != nil {
			t.Fatal(err)
		}
	}
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/verify", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func testServer(ext extraction.Extractor) *Server {
	logger := slog.New(slog.DiscardHandler)
	return NewServer(logger, "", ext, batch.NewManager(ext, logger, 2, time.Hour))
}

const validApp = `{"beverage_type":"distilled_spirits","brand_name":"Stone's Throw","class_type":"Bourbon","alcohol_content":45,"net_contents":"750 mL","name_address":"Bottled by X, Louisville, KY"}`

var verifyStubResult = extraction.Result{
	BrandName:         extraction.Field{Found: true, Value: "Stone's Throw", Confidence: "high"},
	ClassType:         extraction.Field{Found: true, Value: "Bourbon", Confidence: "high"},
	AlcoholContent:    extraction.Field{Found: true, Value: "45% Alc./Vol.", Confidence: "high"},
	NetContents:       extraction.Field{Found: true, Value: "750 mL", Confidence: "high"},
	NameAddress:       extraction.Field{Found: true, Value: "Bottled by X, Louisville, KY", Confidence: "high"},
	GovernmentWarning: extraction.Field{Found: true, Value: verify.CanonicalWarning, Confidence: "high"},
	WarningFormat:     extraction.WarningFormat{HeaderAllCaps: true, HeaderBold: true, Continuous: true, SeparateFromOtherText: true},
}

func TestHandleVerifyOK(t *testing.T) {
	srv := testServer(&stubExtractor{result: &verifyStubResult})

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, multipartRequest(t, validApp, pngBytes(t)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}
	var resp VerifyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != verify.StatusPass { // manual checks don't gate the verdict
		t.Errorf("report status = %s, want pass", resp.Status)
	}
	if len(resp.Results) == 0 || resp.Extraction == nil {
		t.Error("expected rule results and extraction in response")
	}
	if _, ok := resp.TimingsMS["total"]; !ok {
		t.Error("expected timings in response")
	}
}

func TestHandleVerifyValidation(t *testing.T) {
	srv := testServer(&stubExtractor{result: &extraction.Result{}})

	cases := []struct {
		name string
		req  *http.Request
	}{
		{"missing application", multipartRequest(t, "", pngBytes(t))},
		{"bad beverage type", multipartRequest(t, `{"beverage_type":"cider","brand_name":"X"}`, pngBytes(t))},
		{"missing images", multipartRequest(t, validApp)},
		{"non-image upload", multipartRequest(t, validApp, []byte("just text, not an image"))},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, c.req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400 (body %s)", c.name, rec.Code, rec.Body.String())
		}
	}
}

func TestHandleVerifyExtractionFailure(t *testing.T) {
	srv := testServer(&stubExtractor{err: context.DeadlineExceeded})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, multipartRequest(t, validApp, pngBytes(t)))
	if rec.Code != http.StatusBadGateway {
		t.Errorf("status %d, want 502", rec.Code)
	}
}
