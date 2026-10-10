package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"knov/internal/pathutils"
)

// TestMoveRedirectsOnlyViewedFile checks that rename and folder move only send HX-Redirect when
// the HX-Current-URL page shows the moved file, and toast in place (HX-Trigger) otherwise.
func TestMoveRedirectsOnlyViewedFile(t *testing.T) {
	// the server tests share one data dir that lives for the whole run - start from an empty docs folder, so -count=2 works
	os.RemoveAll(pathutils.DocsRoot())
	t.Cleanup(func() { os.RemoveAll(pathutils.DocsRoot()) })
	for _, f := range []string{"a.md", "b.md", "d1/c.md", "d2/c.md"} {
		p := pathutils.GuessMeta(f).FullPath()
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("# x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// every case moves its own source, so each one runs on its own (-run) too
	cases := []struct {
		name, target, form, current, wantRedirect string
		handler                                   http.HandlerFunc
	}{
		{"rename viewed file", "/api/files/rename/a.md", "name=a2.md", "/files/a.md", "/files/a2.md", handleAPIRenameFile},
		{"rename other file", "/api/files/rename/b.md", "name=b2.md", "/files/a.md", "", handleAPIRenameFile},
		{"move folder of viewed file", "/api/files/move-folder/d1", "target=.&name=d1x", "/files/edit/d1/c.md", "/files/d1x/c.md", handleAPIMoveFolderFile},
		{"move folder of other file", "/api/files/move-folder/d2", "target=.&name=d2x", "/files/a.md", "", handleAPIMoveFolderFile},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, c.target, strings.NewReader(c.form))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("HX-Current-URL", (&url.URL{Scheme: "http", Host: "localhost", Path: c.current}).String())
			rec := httptest.NewRecorder()
			c.handler(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("HX-Redirect"); got != c.wantRedirect {
				t.Errorf("HX-Redirect = %q, want %q", got, c.wantRedirect)
			}
			if gotToast := rec.Header().Get("HX-Trigger") != ""; gotToast != (c.wantRedirect == "") {
				t.Errorf("HX-Trigger set = %v, want %v", gotToast, c.wantRedirect == "")
			}
		})
	}
}

func TestGetFileContentMissingFileIs404(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/files/content/missing-file.md", nil)
	rec := httptest.NewRecorder()
	handleAPIGetFileContent(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for a missing file, got %d", rec.Code)
	}
}
