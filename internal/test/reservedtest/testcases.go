package reservedtest

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"

	"knov/internal/files"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/server"
	"knov/internal/test"
)

// request sends method to the real router with form values (nil for none) and returns the
// status and html body.
func request(method, target string, form url.Values) (int, string, error) {
	ts := httptest.NewServer(server.NewRouter())
	defer ts.Close()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, ts.URL+target, body)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Accept", "text/html")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), err
}

func exists(p string) bool {
	_, err := os.Stat(fullPath(p))
	return err == nil
}

// caseListing: the docs listing gives every sample doc its own docs/ path and view url.
func caseListing() test.CaseResult {
	all, err := files.GetAllPhysicalFiles()
	if err != nil {
		return errCase("reserved-listing", err)
	}
	var gaps []string
	for _, top := range reserved {
		want := "docs/" + doc(top, "synced.md")
		i := slices.IndexFunc(all, func(f files.File) bool { return f.Path == want })
		if i == -1 {
			gaps = append(gaps, fmt.Sprintf("%q not listed", want))
			continue
		}
		if got := all[i].ViewURL(); got != pathutils.ToFileURL(doc(top, "synced.md")) {
			gaps = append(gaps, fmt.Sprintf("%q: view url %q", want, got))
		}
	}
	return gapsCase("reserved-listing", "docs files in docs/docs/, docs/media/ and docs/files/ are listed as themselves", gaps)
}

// caseView: the file page, its raw content and its edit page show the doc, not its partner.
func caseView() test.CaseResult {
	var gaps []string
	for _, top := range reserved {
		rel := doc(top, "synced.md")
		for _, target := range []string{pathutils.ToFileURL(rel), "/api/files/raw?filepath=" + url.QueryEscape(rel), "/api/files/content/" + strings.TrimPrefix(pathutils.ToFileURL(rel), "/files/")} {
			status, body, err := request(http.MethodGet, target, nil)
			if err != nil {
				return errCase("reserved-view", err)
			}
			if status != http.StatusOK || !strings.Contains(body, marker(top)) || strings.Contains(body, partnerMarker(top)) {
				gaps = append(gaps, fmt.Sprintf("%s: status %d, shows the doc %v, its partner %v", target, status, strings.Contains(body, marker(top)), strings.Contains(body, partnerMarker(top))))
			}
		}
	}
	return gapsCase("reserved-view", "the file page, raw content and content api of each sample doc show it, not its collision partner", gaps)
}

// caseMetadata: each sample doc and its partner have their own metadata.
func caseMetadata() test.CaseResult {
	var gaps []string
	for _, top := range reserved {
		key, other := "docs/"+doc(top, "synced.md"), partner(top, "synced.md")
		m, err := files.MetaDataGet(key)
		if err != nil || m == nil || m.Path != key {
			gaps = append(gaps, fmt.Sprintf("%q: metadata %+v, %v", key, m, err))
			continue
		}
		if err := files.MetaDataMutate(key, func(m *files.Metadata, _ bool) (bool, error) {
			m.Tags = []string{"reserved-" + top}
			return true, nil
		}); err != nil {
			return errCase("reserved-metadata", err)
		}
		if o, err := files.MetaDataGet(other); err != nil || o == nil || o.Path != other || slices.Contains(o.Tags, "reserved-"+top) {
			gaps = append(gaps, fmt.Sprintf("%q: partner metadata %+v, %v", other, o, err))
		}
	}
	return gapsCase("reserved-metadata", "a sample doc and its collision partner don't share metadata", gaps)
}

// caseLinks: a docs/-prefixed wikilink and /files/docs/ url from the docs root and a bare link from
// the doc's own folder read and render as the sample doc.
func caseLinks() test.CaseResult {
	var gaps []string
	for _, top := range reserved {
		want := "docs/" + doc(top, "synced.md")
		forms := map[string][2]string{
			"wiki":     {"docs/" + sub + "/linker.md", parser.Link{Kind: parser.LinkWiki, Path: want}.String()},
			"file url": {"docs/" + sub + "/linker.md", "[x](" + parser.Link{Kind: parser.LinkMarkdown, Path: "/files/" + want}.Dest() + ")"},
			"bare":     {"docs/" + doc(top, "linker.md"), "[x](synced.md)"},
		}
		for form, f := range forms {
			got := (&parser.MarkdownHandler{}).ExtractLinks([]byte(f[1]), f[0])
			rendered := parser.RenderLinks(f[1], f[0])
			if !slices.Equal(got, []string{want}) || !strings.Contains(rendered, "("+pathutils.ToFileURL(doc(top, "synced.md"))+")") {
				gaps = append(gaps, fmt.Sprintf("%s %q in %s: reads %q, renders %q", form, f[1], f[0], got, rendered))
			}
		}
	}
	return gapsCase("reserved-links", "links to a sample doc read and render as it", gaps)
}

// caseCreate: saving a new doc into each reserved folder through the api creates it there.
func caseCreate() test.CaseResult {
	var gaps []string
	for _, top := range reserved {
		rel := doc(top, "new.md")
		status, body, err := request(http.MethodPost, "/api/files/save", url.Values{"filepath": {rel}, "content": {marker(top)}, "editor": {"codemirror"}})
		if err != nil {
			return errCase("reserved-create", err)
		}
		if status != http.StatusOK || !exists("docs/"+rel) || exists(partner(top, "new.md")) {
			gaps = append(gaps, fmt.Sprintf("save %q: status %d (%s), created %v, partner created %v", rel, status, strings.TrimSpace(body), exists("docs/"+rel), exists(partner(top, "new.md"))))
		}
	}
	return gapsCase("reserved-create", "a new doc saved into docs/docs/, docs/media/ and docs/files/ lands there", gaps)
}

// caseRenameMoveDelete: the doc created by caseCreate is renamed, moved into the next reserved
// folder and deleted through the api, its partner is never touched.
func caseRenameMoveDelete() test.CaseResult {
	var gaps []string
	for i, top := range reserved {
		next := reserved[(i+1)%len(reserved)]
		steps := []struct {
			method, target string
			form           url.Values
			gone, there    string
		}{
			{http.MethodPost, pathutils.ToRouteURL("/api/files/rename/", doc(top, "new.md")), url.Values{"name": {doc(top, "renamed.md")}}, doc(top, "new.md"), doc(top, "renamed.md")},
			{http.MethodPost, pathutils.ToRouteURL("/api/files/rename/", doc(top, "renamed.md")), url.Values{"name": {doc(next, "moved-"+top+".md")}}, doc(top, "renamed.md"), doc(next, "moved-"+top+".md")},
			{http.MethodDelete, pathutils.ToRouteURL("/api/files/delete/", doc(next, "moved-"+top+".md")), nil, doc(next, "moved-"+top+".md"), ""},
		}
		for _, st := range steps {
			status, body, err := request(st.method, st.target, st.form)
			if err != nil {
				return errCase("reserved-rename-move-delete", err)
			}
			if status != http.StatusOK || exists("docs/"+st.gone) || (st.there != "" && !exists("docs/"+st.there)) {
				gaps = append(gaps, fmt.Sprintf("%s %s: status %d (%s), source left %v, target there %v", st.method, st.target, status, strings.TrimSpace(body), exists("docs/"+st.gone), st.there == "" || exists("docs/"+st.there)))
				break
			}
		}
		if !exists(partner(top, "synced.md")) {
			gaps = append(gaps, fmt.Sprintf("%q: partner gone", partner(top, "synced.md")))
		}
	}
	return gapsCase("reserved-rename-move-delete", "a doc in a reserved folder is renamed, moved and deleted as itself", gaps)
}
