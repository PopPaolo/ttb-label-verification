package batch

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"log/slog"
	"testing"
	"time"

	"ttb-label-verification/internal/extraction"
	"ttb-label-verification/internal/verify"
)

type stubExtractor struct{}

func (stubExtractor) Extract(_ context.Context, _ []extraction.Image) (*extraction.Result, error) {
	return &extraction.Result{
		BrandName:         extraction.Field{Found: true, Value: "Hop Test", Confidence: "high"},
		ClassType:         extraction.Field{Found: true, Value: "India Pale Ale", Confidence: "high"},
		AlcoholContent:    extraction.Field{Found: true, Value: "6.5% ALC/VOL", Confidence: "high"},
		NetContents:       extraction.Field{Found: true, Value: "12 FL. OZ.", Confidence: "high"},
		NameAddress:       extraction.Field{Found: true, Value: "Brewed by Hop Test, Portland, OR", Confidence: "high"},
		GovernmentWarning: extraction.Field{Found: true, Value: verify.CanonicalWarning, Confidence: "high"},
		WarningFormat:     extraction.WarningFormat{HeaderAllCaps: true, HeaderBold: true, Continuous: true, SeparateFromOtherText: true},
	}, nil
}

func testZip(t *testing.T, man manifest, images map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	mw, err := zw.Create("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(mw).Encode(man); err != nil {
		t.Fatal(err)
	}
	for name, data := range images {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	_ = zw.Close()
	return buf.Bytes()
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func maltApp(id string) ManifestApp {
	return ManifestApp{
		ID:     id,
		Images: []string{id + "/label.png"},
		Application: verify.Application{
			BeverageType: verify.Malt, BrandName: "Hop Test", ClassType: "India Pale Ale",
			AlcoholContent: 6.5, NetContents: "12 fl oz", NameAddress: "Brewed by Hop Test, Portland, OR",
		},
	}
}

func waitSettled(t *testing.T, m *Manager, id string) Summary {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s, ok := m.Get(id)
		if !ok {
			t.Fatal("batch disappeared")
		}
		if s.Pending == 0 {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("batch did not settle in time")
	return Summary{}
}

func TestBatchEndToEnd(t *testing.T) {
	m := NewManager(stubExtractor{}, slog.New(slog.DiscardHandler), 4, time.Hour)

	good := maltApp("app-001")
	missingImage := maltApp("app-002")
	missingImage.Images = []string{"app-002/nonexistent.png"}

	zipData := testZip(t, manifest{Applications: []ManifestApp{good, missingImage}},
		map[string][]byte{"app-001/label.png": pngBytes(t)})

	b, err := m.Submit(zipData)
	if err != nil {
		t.Fatal(err)
	}

	s := waitSettled(t, m, b.ID)
	if s.Total != 2 {
		t.Fatalf("total = %d, want 2", s.Total)
	}
	byID := map[string]ItemSummary{}
	for _, it := range s.Items {
		byID[it.ID] = it
	}
	// Partial failure (D7): the missing-image item fails, the other proceeds.
	if byID["app-002"].Status != ItemFailed {
		t.Errorf("app-002 = %s, want failed", byID["app-002"].Status)
	}
	if byID["app-001"].Status != ItemDone {
		t.Errorf("app-001 = %s (%s), want done", byID["app-001"].Status, byID["app-001"].Error)
	}

	item, ok := m.GetItem(b.ID, "app-001")
	if !ok || item.Report == nil || item.Extraction == nil {
		t.Fatal("expected full report for app-001")
	}
	if len(item.images) != 0 {
		t.Error("images should be discarded after processing (D8)")
	}
}

func TestBatchRejectsBadArchives(t *testing.T) {
	m := NewManager(stubExtractor{}, slog.New(slog.DiscardHandler), 1, time.Hour)

	if _, err := m.Submit([]byte("not a zip")); err == nil {
		t.Error("non-zip accepted")
	}

	noManifest := testZip(t, manifest{}, nil)
	// testZip always writes a manifest; build one without it manually.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fw, _ := zw.Create("something.png")
	_, _ = fw.Write(pngBytes(t))
	_ = zw.Close()
	if _, err := m.Submit(buf.Bytes()); err == nil {
		t.Error("archive without manifest accepted")
	}

	if _, err := m.Submit(noManifest); err == nil {
		t.Error("empty applications list accepted")
	}

	dup := testZip(t, manifest{Applications: []ManifestApp{maltApp("same"), maltApp("same")}},
		map[string][]byte{"same/label.png": pngBytes(t)})
	if _, err := m.Submit(dup); err == nil {
		t.Error("duplicate ids accepted")
	}
}
