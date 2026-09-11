package server

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// malformed "aligns" must be rejected with a real 4xx before any file is touched -
// same contract as the existing headers/rows JSON fields.
func TestHandleAPITableEditorSaveRejectsMalformedAligns(t *testing.T) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fields := map[string]string{
		"filepath":   "does-not-matter.md",
		"headers":    `["A","B"]`,
		"rows":       `[["1","2"]]`,
		"aligns":     `not-json`,
		"tableIndex": "0",
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/editor/tableeditor", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()

	handleAPITableEditorSave(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
