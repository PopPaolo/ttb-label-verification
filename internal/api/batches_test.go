package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ttb-label-verification/internal/batch"
	"ttb-label-verification/internal/verify"
)

func batchZip(t *testing.T) []byte {
	t.Helper()
	man := map[string]any{"applications": []map[string]any{{
		"id":            "app-001",
		"images":        []string{"label.png"},
		"beverage_type": verify.DistilledSpirits,
		"brand_name":    "Stone's Throw",
		"class_type":    "Bourbon",
	}}}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	mw, _ := zw.Create("manifest.json")
	_ = json.NewEncoder(mw).Encode(man)
	fw, _ := zw.Create("label.png")
	_, _ = fw.Write(pngBytes(t))
	_ = zw.Close()
	return buf.Bytes()
}

func TestBatchEndpoints(t *testing.T) {
	srv := testServer(&stubExtractor{result: &verifyStubResult})

	// Submit as raw zip body.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/batches", bytes.NewReader(batchZip(t)))
	req.Header.Set("Content-Type", "application/zip")
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("submit status %d, body %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("bad submit response: %s", rec.Body.String())
	}

	// Poll status until settled.
	deadline := time.Now().Add(5 * time.Second)
	var summary batch.Summary
	for {
		rec = httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/batches/"+created.ID, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status endpoint %d", rec.Code)
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
			t.Fatal(err)
		}
		if summary.Pending == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("batch did not settle")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Item detail.
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/batches/"+created.ID+"/items/app-001", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("item endpoint %d, body %s", rec.Code, rec.Body.String())
	}

	// Unknown batch 404s.
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/batches/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown batch = %d, want 404", rec.Code)
	}
}
