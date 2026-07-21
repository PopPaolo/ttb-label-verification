package api

import (
	"io"
	"net/http"
)

// maxBatchBytes caps the zip upload — the batch endpoint is unauthenticated
// and each label triggers a paid extraction call, so the cap doubles as abuse
// mitigation (D6a).
const maxBatchBytes = 250 << 20

// handleBatchSubmit implements POST /api/batches: a multipart form with a
// `batch` zip file (or a raw application/zip body). Returns the batch ID
// immediately; processing is asynchronous (D7).
func (s *Server) handleBatchSubmit(w http.ResponseWriter, r *http.Request) {
	zipData, err := readBatchUpload(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{err.Error()})
		return
	}

	b, err := s.batches.Submit(zipData)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"id": b.ID})
}

func readBatchUpload(r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBatchBytes)

	// Multipart form with a `batch` file (what the frontend sends)…
	if err := r.ParseMultipartForm(maxBatchBytes); err == nil {
		defer func() { _ = r.MultipartForm.RemoveAll() }()
		if files := r.MultipartForm.File["batch"]; len(files) > 0 {
			f, err := files[0].Open()
			if err != nil {
				return nil, errBadUpload
			}
			defer f.Close()
			return io.ReadAll(f)
		}
		return nil, errBadUpload
	}

	// …or a raw zip body (curl --data-binary convenience).
	data, err := io.ReadAll(r.Body)
	if err != nil || len(data) == 0 {
		return nil, errBadUpload
	}
	return data, nil
}

var errBadUpload = &uploadError{}

type uploadError struct{}

func (*uploadError) Error() string {
	return "send the batch as a zip file: multipart field 'batch', or a raw application/zip body (max 250 MB)"
}

// handleBatchStatus implements GET /api/batches/{id}: triage summary.
func (s *Server) handleBatchStatus(w http.ResponseWriter, r *http.Request) {
	summary, ok := s.batches.Get(r.PathValue("id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{"batch not found — batches are kept in memory and expire after a few hours"})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// handleBatchItem implements GET /api/batches/{id}/items/{itemID}: the full
// per-label report, same shape as /api/verify.
func (s *Server) handleBatchItem(w http.ResponseWriter, r *http.Request) {
	item, ok := s.batches.GetItem(r.PathValue("id"), r.PathValue("itemID"))
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{"batch or item not found"})
		return
	}
	writeJSON(w, http.StatusOK, item)
}
