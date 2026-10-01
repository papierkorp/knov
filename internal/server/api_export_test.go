package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"knov/internal/configmanager"
	"knov/internal/export"
)

func TestExportHandlers(t *testing.T) {
	prev := configmanager.GetAppConfig()
	t.Cleanup(func() { configmanager.SetDataAndStoragePaths(prev.DataPath, prev.StoragePath) })
	configmanager.SetDataAndStoragePaths(t.TempDir(), t.TempDir())

	r := chi.NewRouter()
	r.Get("/api/exports/files", handleAPIExportFiles)
	r.Get("/api/exports/pdf", handleAPIDownloadPDFExport)
	r.Delete("/api/exports/pdf", handleAPIDeleteExport)
	do := func(method, kind string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(method, "/api/exports/"+kind, nil))
		return rec
	}

	if rec := do(http.MethodGet, export.KindFiles); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/zip" {
		t.Errorf("stream files: status = %d content-type = %q, want 200 zip", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec := do(http.MethodGet, export.KindPDF); rec.Code != http.StatusNotFound {
		t.Errorf("download without pdf archive: status = %d, want 404", rec.Code)
	}

	if err := os.MkdirAll(filepath.Dir(export.Path()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(export.Path(), []byte("zip"), 0644); err != nil {
		t.Fatal(err)
	}
	if rec := do(http.MethodGet, export.KindPDF); rec.Code != http.StatusOK || rec.Body.String() != "zip" {
		t.Errorf("download pdf: status = %d body = %q, want 200 with the archive", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodDelete, export.KindPDF); rec.Code != http.StatusOK {
		t.Errorf("delete: status = %d, want 200", rec.Code)
	}
	if export.Available() {
		t.Error("archive still available after delete")
	}
}
