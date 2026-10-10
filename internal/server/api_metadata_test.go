package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// the json body decodes straight into the typed paths, so a path or parent without its docs/ or
// media/ prefix is a 400 instead of a metadata record under a key no file has
func TestSetMetadataRejectsNonMetaPaths(t *testing.T) {
	for _, body := range []string{
		`{"path": "notes.md"}`,
		`{"path": "docs/../notes.md"}`,
		`{"path": "docs/notes.md", "parents": ["parent.md"]}`,
	} {
		w := httptest.NewRecorder()
		handleAPISetMetadata(w, httptest.NewRequest(http.MethodPost, "/api/metadata", strings.NewReader(body)))
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", body, w.Code)
		}
	}
}
