package metadatatest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"knov/internal/files"
	"knov/internal/pathutils"
	"knov/internal/server"
	"knov/internal/test"
)

// specialCharFile has a non-ASCII char in its name so it exercises the percent-encoding
// round-trip. location.pathname in the browser serves "ö" as "%C3%B6", and panel-file.js
// used to encodeURIComponent that already-encoded segment a second time, so every
// ?filepath= lookup arrived as the literal "tr%C3%B6te.md" and missed the stored row.
const specialCharFile = "tröte.md"

// caseGetMetadataSpecialCharFilepath serves the real router and hits GET /api/metadata for a
// file whose name contains "ö": once with the correctly encoded query value (must resolve),
// once with the double-encoded value the old JS produced (must 404). Locks the server
// contract the panel-file.js fix depends on - a byte-exact filepath must round-trip through
// the query param and ToWithPrefix into storage.
func caseGetMetadataSpecialCharFilepath() test.CaseResult {
	name := "get-metadata-special-char-filepath"

	relPath := testPath(specialCharFile) // test/metadata-tests/tröte.md
	if err := writeFile(relPath, "# "+specialCharFile+"\n\ncontent\n"); err != nil {
		return errCase(name, err)
	}
	if err := files.MetaDataSync(pathutils.ToWithPrefix(relPath)); err != nil {
		return errCase(name, err)
	}
	defer func() { _ = files.MetaDataDelete(pathutils.ToWithPrefix(relPath)) }()

	ts := httptest.NewServer(server.NewRouter())
	defer ts.Close()

	get := func(filepathParam string) (int, string) {
		u := ts.URL + "/api/metadata?" + url.Values{"filepath": {filepathParam}}.Encode()
		req, _ := http.NewRequest(http.MethodGet, u, nil)
		req.Header.Set("Accept", "application/json")
		resp, err := ts.Client().Do(req)
		if err != nil {
			return -1, err.Error()
		}
		defer resp.Body.Close()
		buf := make([]byte, 512)
		n, _ := resp.Body.Read(buf)
		return resp.StatusCode, string(buf[:n])
	}

	// correctly encoded: raw path -> url.Values.Encode adds one layer -> server decodes to relPath
	okStatus, okBody := get(pathutils.DocsPath(relPath).String())
	// double-encoded: the "ö" already came in as "%C3%B6" (as location.pathname serves it),
	// then got encoded again -> server decodes to a literal-percent path that no row matches
	bugStatus, _ := get(pathutils.DocsPath(strings.ReplaceAll(relPath, "ö", "%C3%B6")).String())

	resolved := okStatus == http.StatusOK && strings.Contains(okBody, "tröte.md")
	doubleEncoded404 := bugStatus == http.StatusNotFound

	success := resolved && doubleEncoded404
	cr := test.CaseResult{
		Name:     name,
		Expected: "correctly-encoded filepath -> 200 with the file's metadata; double-encoded filepath -> 404",
		Actual:   fmt.Sprintf("correct=%d double-encoded=%d", okStatus, bugStatus),
		Success:  success,
	}
	if !success {
		cr.Error = "GET /api/metadata did not round-trip a special-char filepath as expected"
	}
	return cr
}
