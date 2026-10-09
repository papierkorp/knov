package reservedtest

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"knov/internal/book"
	"knov/internal/configStorage"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/server"
	"knov/internal/server/render"
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
		if got := all[i].ViewURL(); got != pathutils.ToFileURL("docs/"+doc(top, "synced.md")) {
			gaps = append(gaps, fmt.Sprintf("%q: view url %q", want, got))
		}
	}
	return gapsCase("reserved-listing", "docs files in docs/docs/, docs/media/ and docs/files/ are listed as themselves", gaps)
}

// caseView: the file page and its content api show the doc, not its partner.
func caseView() test.CaseResult {
	var gaps []string
	for _, top := range reserved {
		rel := doc(top, "synced.md")
		for _, target := range []string{pathutils.ToFileURL("docs/" + rel), "/api/files/content/" + rel} {
			status, body, err := request(http.MethodGet, target, nil)
			if err != nil {
				return errCase("reserved-view", err)
			}
			if status != http.StatusOK || !strings.Contains(body, marker(top)) || strings.Contains(body, partnerMarker(top)) {
				gaps = append(gaps, fmt.Sprintf("%s: status %d, shows the doc %v, its partner %v", target, status, strings.Contains(body, marker(top)), strings.Contains(body, partnerMarker(top))))
			}
		}
	}
	// the edit page names the doc by its docs/ path
	for _, top := range reserved {
		rel := doc(top, "synced.md")
		for _, target := range []string{pathutils.ToFileEditURL("docs/" + rel)} {
			status, body, err := request(http.MethodGet, target, nil)
			if err != nil {
				return errCase("reserved-view", err)
			}
			if status != http.StatusOK || !strings.Contains(body, "filepath="+url.QueryEscape("docs/"+rel)) {
				gaps = append(gaps, fmt.Sprintf("%s: status %d, does not name %q", target, status, "docs/"+rel))
			}
		}
	}
	return gapsCase("reserved-view", "the file page and content api of each sample doc show it, not its collision partner", gaps)
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

// caseLinks: a docs file in docs/docs/, docs/media/ or docs/files/ is linked as itself - by a
// /files/ url (taken literally), a docs/-prefixed wikilink and a bare link from its own folder -
// and a link written media/... still reads as the media file.
func caseLinks() test.CaseResult {
	var gaps []string
	for _, top := range reserved {
		want := "docs/" + doc(top, "synced.md")
		forms := map[string][3]string{
			"wiki":       {"docs/" + sub + "/linker.md", parser.Link{Kind: parser.LinkWiki, Path: want}.String(), want},
			"file url":   {"docs/" + sub + "/linker.md", "[x](" + parser.Link{Kind: parser.LinkMarkdown, Path: "/files/" + doc(top, "synced.md")}.Dest() + ")", want},
			"html":       {"docs/" + sub + "/linker.md", `<a href="` + pathutils.ToFileURL(want) + `">x</a>`, want},
			"bare":       {"docs/" + doc(top, "linker.md"), "[x](synced.md)", want},
			"dot":        {"docs/" + doc(top, "linker.md"), "[x](./synced.md)", want},
			"wiki media": {"docs/" + sub + "/linker.md", parser.Link{Kind: parser.LinkWiki, Path: "media/" + sub + "/synced.md"}.String(), "media/" + sub + "/synced.md"},
		}
		for form, f := range forms {
			got := (&parser.MarkdownHandler{}).ExtractLinks([]byte(f[1]), f[0])
			if !slices.Equal(got, []string{f[2]}) {
				gaps = append(gaps, fmt.Sprintf("%s %q in %s: reads %q, want %q", form, f[1], f[0], got, f[2]))
			}
			if rendered := parser.RenderLinks(f[1], f[0]); f[2] == want && !strings.Contains(rendered, pathutils.ToFileURL(want)) {
				gaps = append(gaps, fmt.Sprintf("%s %q in %s: renders %q", form, f[1], f[0], rendered))
			}
		}
	}
	return gapsCase("reserved-links", "links to a sample doc read and render as it, media/ links stay media", gaps)
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

// caseDelete: deleting a doc through the api removes it, not its partner.
func caseDelete() test.CaseResult {
	var gaps []string
	for _, top := range reserved {
		rel := doc(top, "delete.md")
		if err := write(fullPath("docs/"+rel), "# delete\n"); err != nil {
			return errCase("reserved-delete", err)
		}
		if err := write(fullPath(partner(top, "delete.md")), "# partner\n"); err != nil {
			return errCase("reserved-delete", err)
		}
		status, body, err := request(http.MethodDelete, pathutils.ToRouteURL("/api/files/delete/", rel), nil)
		if err != nil {
			return errCase("reserved-delete", err)
		}
		if status != http.StatusOK || exists("docs/"+rel) || !exists(partner(top, "delete.md")) {
			gaps = append(gaps, fmt.Sprintf("delete %q: status %d (%s), doc left %v, partner there %v", rel, status, strings.TrimSpace(body), exists("docs/"+rel), exists(partner(top, "delete.md"))))
		}
	}
	return gapsCase("reserved-delete", "deleting a doc in a reserved folder removes it, not its collision partner", gaps)
}

// caseParams: a query / form param naming an existing file is its metadata path - the sample doc
// is read through it (raw content, editor, metadata api, links api), never its collision
// partner, and the unprefixed docs-relative form is answered with 400.
func caseParams() test.CaseResult {
	var gaps []string
	for _, top := range reserved {
		rel, meta := doc(top, "synced.md"), "docs/"+doc(top, "synced.md")
		for _, target := range []string{"/api/files/raw?filepath=" + url.QueryEscape(meta), "/api/editor?filepath=" + url.QueryEscape(meta)} {
			status, body, err := request(http.MethodGet, target, nil)
			if err != nil {
				return errCase("reserved-params", err)
			}
			if status != http.StatusOK || !strings.Contains(body, marker(top)) || strings.Contains(body, partnerMarker(top)) {
				gaps = append(gaps, fmt.Sprintf("%s: status %d, shows the doc %v, its partner %v", target, status, strings.Contains(body, marker(top)), strings.Contains(body, partnerMarker(top))))
			}
		}
		status, body, err := request(http.MethodGet, "/api/metadata?filepath="+url.QueryEscape(meta), nil)
		if err != nil {
			return errCase("reserved-params", err)
		}
		if status != http.StatusOK || !strings.Contains(body, meta) {
			gaps = append(gaps, fmt.Sprintf("metadata of %q: status %d (%s)", meta, status, strings.TrimSpace(body)))
		}
		// for docs/ and media/ the docs-relative path is itself a metadata path
		for _, target := range []string{"/api/files/raw?filepath=" + url.QueryEscape(rel), "/api/metadata?filepath=" + url.QueryEscape(rel)} {
			if pathutils.IsMetaPath(rel) {
				break
			}
			if status, _, err := request(http.MethodGet, target, nil); err != nil || status != http.StatusBadRequest {
				gaps = append(gaps, fmt.Sprintf("%s: status %d, want 400 (%v)", target, status, err))
			}
		}
		// saving through the editor form edits the doc itself
		if status, body, err := request(http.MethodPost, "/api/files/save", url.Values{"filepath": {rel}, "content": {"# synced\n\n" + marker(top) + "\n\nedited\n"}}); err != nil || status != http.StatusOK {
			gaps = append(gaps, fmt.Sprintf("save %q: status %d (%s) %v", rel, status, strings.TrimSpace(body), err))
		} else if b, _ := os.ReadFile(fullPath(meta)); !strings.Contains(string(b), "edited") {
			gaps = append(gaps, fmt.Sprintf("save %q: doc not changed", rel))
		}
		if b, _ := os.ReadFile(fullPath(partner(top, "synced.md"))); strings.Contains(string(b), "edited") {
			gaps = append(gaps, fmt.Sprintf("save %q: partner changed", rel))
		}
	}
	return gapsCase("reserved-params", "file params are metadata paths, read and saved as the sample doc, not its collision partner", gaps)
}

// caseStoredPaths: the parents of a sample doc and a filter on its folder and parent keep the
// docs/ path - the doc in docs/media/ is not read as the media file, a parent without prefix is
// answered with 400.
func caseStoredPaths() test.CaseResult {
	var gaps []string
	for i, top := range reserved {
		meta, parent := "docs/"+doc(top, "synced.md"), "docs/"+doc(reserved[(i+1)%len(reserved)], "synced.md")
		status, body, err := request(http.MethodPost, "/api/metadata/parents", url.Values{"filepath": {meta}, "parents": {parent}})
		if err != nil {
			return errCase("reserved-stored-paths", err)
		}
		if m, _ := files.MetaDataGet(meta); status != http.StatusOK || m == nil || !slices.Equal(m.Parents, []string{parent}) {
			gaps = append(gaps, fmt.Sprintf("parents of %q: status %d (%s), stored %+v", meta, status, strings.TrimSpace(body), m))
		}
		// for docs/ and media/ the docs-relative path is itself a metadata path
		if bare := doc(reserved[(i+1)%len(reserved)], "synced.md"); !pathutils.IsMetaPath(bare) {
			if status, _, err := request(http.MethodPost, "/api/metadata/parents", url.Values{"filepath": {meta}, "parents": {bare}}); err != nil || status != http.StatusBadRequest {
				gaps = append(gaps, fmt.Sprintf("parents of %q without prefix: status %d, want 400 (%v)", meta, status, err))
			}
		}
		for _, c := range []filter.Criteria{
			{Metadata: "collection", Operator: "equals", Value: top, Action: "include"},
			{Metadata: "child-of", Operator: "equals", Value: parent, Action: "include"},
		} {
			all, err := files.GetAllPhysicalFiles()
			if err != nil {
				return errCase("reserved-stored-paths", err)
			}
			matched := filter.FilterFileList(all, []filter.Criteria{c}, "and")
			if !slices.ContainsFunc(matched, func(f files.File) bool { return f.Path == meta }) || slices.ContainsFunc(matched, func(f files.File) bool { return f.Path == partner(top, "synced.md") }) {
				gaps = append(gaps, fmt.Sprintf("filter %s=%q: matches %d files, the doc %q not found or its partner found", c.Metadata, c.Value, len(matched), meta))
			}
		}
	}
	return gapsCase("reserved-stored-paths", "parents and filters on a sample doc keep its docs/ path", gaps)
}

// caseMigration: the metadata record a docs file in a reserved folder had under its old key (the
// path without docs/) moves to its own key unless a file lives at the old key, and a saved filter
// named like a reserved folder (media/x) is renamed to the id its paired file x.index belongs to.
func caseMigration() test.CaseResult {
	var gaps []string
	// docs/docs/x.md and docs/files/x.md shared one old key (docs/x.md), the first one found keeps the record
	for _, top := range []string{"media", "docs"} {
		rel := doc(top, "migrate.md")
		legacy, key := pathutils.ToWithPrefix(rel), "docs/"+rel
		if err := write(fullPath(key), "# migrate\n"); err != nil {
			return errCase("reserved-migration", err)
		}
		if err := files.MetaDataMutate(legacy, func(m *files.Metadata, _ bool) (bool, error) {
			m.Tags = []string{"legacy-" + top}
			return true, nil
		}); err != nil {
			return errCase("reserved-migration", err)
		}
	}
	if status, body, err := request(http.MethodPost, "/api/metadata/reserved-folders/migrate", nil); err != nil || status != http.StatusOK {
		return errCase("reserved-migration", fmt.Errorf("migrate endpoint: %d %v %s", status, err, body))
	}
	for _, top := range []string{"media", "docs"} {
		rel := doc(top, "migrate.md")
		legacy, key := pathutils.ToWithPrefix(rel), "docs/"+rel
		if m, _ := files.MetaDataGet(key); m == nil || !slices.Contains(m.Tags, "legacy-"+top) {
			gaps = append(gaps, fmt.Sprintf("%q: record not moved from %q: %+v", key, legacy, m))
		}
		if m, _ := files.MetaDataGet(legacy); m != nil && !exists(legacy) {
			gaps = append(gaps, fmt.Sprintf("%q: old record left", legacy))
		}
		_ = files.MetaDataDelete(key)
	}

	for _, top := range reserved {
		id := "knov-test-migrate-" + top
		paired := filter.FilterIndexPath(id)
		if err := write(fullPath("docs/"+paired), "# filter\n"); err != nil {
			return errCase("reserved-migration", err)
		}
		defer os.Remove(fullPath("docs/" + paired))
		if err := configStorage.Set("filter/"+top+"/"+id, []byte("{}")); err != nil {
			return errCase("reserved-migration", err)
		}
		defer configStorage.Delete("filter/" + id)
	}
	if err := filter.MigrateReservedIDs(); err != nil {
		return errCase("reserved-migration", err)
	}
	for _, top := range reserved {
		id := "knov-test-migrate-" + top
		if got, _ := configStorage.Get("filter/" + id); got == nil {
			gaps = append(gaps, fmt.Sprintf("filter id %q: not renamed to %q", top+"/"+id, id))
		}
		if got, _ := configStorage.Get("filter/" + top + "/" + id); got != nil {
			gaps = append(gaps, fmt.Sprintf("filter id %q: old id left", top+"/"+id))
		}
	}
	return gapsCase("reserved-migration", "legacy metadata records and filter ids named like a reserved folder are migrated", gaps)
}

// caseLinkRename: renaming a docs file in a reserved folder rewrites the links to it, in each form
// they were written, to the new file - not to the media or docs file of the old name.
func caseLinkRename() test.CaseResult {
	var gaps []string
	for _, top := range reserved {
		src, dst, linker := "docs/"+doc(top, "rsrc.md"), "docs/"+doc(top, "rdst.md"), "docs/"+doc(top, "rlinker.md")
		content := strings.Join([]string{
			"[a](" + parser.Link{Kind: parser.LinkMarkdown, Path: "/files/" + doc(top, "rsrc.md")}.Dest() + ")",
			parser.Link{Kind: parser.LinkWiki, Path: src}.String(),
			"[c](rsrc.md)",
			"[d](./rsrc.md)",
		}, "\n") + "\n"
		for p, c := range map[string]string{src: "# src\n", linker: content} {
			if err := write(fullPath(p), c); err != nil {
				return errCase("reserved-link-rename", err)
			}
			if err := files.MetaDataSync(p); err != nil {
				return errCase("reserved-link-rename", err)
			}
		}
		if err := files.UpdateLinksForSingleFile(linker); err != nil {
			return errCase("reserved-link-rename", err)
		}
		if err := files.MoveFileNoRefresh(logging.KeyApp, pathutils.DocsPath(strings.TrimPrefix(src, "docs/")), pathutils.DocsPath(strings.TrimPrefix(dst, "docs/"))); err != nil {
			gaps = append(gaps, fmt.Sprintf("rename %q: %v", src, err))
			continue
		}
		data, err := os.ReadFile(fullPath(linker))
		if err != nil {
			return errCase("reserved-link-rename", err)
		}
		got := (&parser.MarkdownHandler{}).ExtractLinks(data, linker)
		if want := []string{dst, dst, dst, dst}; !slices.Equal(got, want) {
			gaps = append(gaps, fmt.Sprintf("links of %q after renaming %q: %q, want %q\n%s", linker, src, got, want, data))
		}
	}
	return gapsCase("reserved-link-rename", "renaming a doc in a reserved folder rewrites every form of its links to the new file", gaps)
}

// caseTree: the file tree names each sample doc by its docs/ path, so its link, rename and delete
// buttons address the doc, not its collision partner.
func caseTree() test.CaseResult {
	var gaps []string
	all, err := files.GetAllPhysicalFiles()
	if err != nil {
		return errCase("reserved-tree", err)
	}
	var sampleDocs []files.File
	for _, f := range all {
		if strings.Contains(f.Path, "/"+sub+"/synced.md") || f.Path == "docs/"+sub+"/synced.md" {
			sampleDocs = append(sampleDocs, f)
		}
	}
	rendered := render.RenderTreeOverview(files.BuildFileTree(sampleDocs), true)
	for _, top := range reserved {
		rel := doc(top, "synced.md")
		for _, want := range []string{`data-path="` + rel + `"`, `href="` + pathutils.ToFileURL("docs/"+rel) + `"`, `hx-delete="` + pathutils.ToRouteURL("/api/files/delete/", rel) + `?inline=true"`} {
			if !strings.Contains(rendered, want) {
				gaps = append(gaps, fmt.Sprintf("tree of %q lacks %s", rel, want))
			}
		}
	}
	return gapsCase("reserved-tree", "the file tree addresses each sample doc by its own path", gaps)
}

// caseBook: a book entry [[docs/media/x]] or [[docs/docs/x]] includes that doc, a media/ entry is no doc.
func caseBook() test.CaseResult {
	var gaps []string
	const bookRel = sub + "/reserved.book"
	var entries []string
	for _, top := range reserved {
		entries = append(entries, "[[docs/"+doc(top, "synced.md")+"]]")
	}
	if err := write(fullPath("docs/"+bookRel), strings.Join(entries, "\n")+"\n"); err != nil {
		return errCase("reserved-book", err)
	}
	composed, err := book.Compose("docs/" + bookRel)
	if err != nil {
		return errCase("reserved-book", err)
	}
	for _, top := range reserved {
		if !strings.Contains(composed, marker(top)) || strings.Contains(composed, partnerMarker(top)) {
			gaps = append(gaps, fmt.Sprintf("book entry docs/%s: includes the doc %v, its partner %v", doc(top, "synced.md"), strings.Contains(composed, marker(top)), strings.Contains(composed, partnerMarker(top))))
		}
	}
	return gapsCase("reserved-book", "book entries name docs in reserved folders with their docs/ prefix", gaps)
}

// caseUpload: uploading from a doc in docs/media/ mirrors the doc's folder in the media folder
// (media/media/...), not in its root.
func caseUpload() test.CaseResult {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("file", "pic.png")
	_, _ = part.Write([]byte("\x89PNG\r\n\x1a\n"))
	_ = w.WriteField("context_path", pathutils.ToFileEditURL("docs/"+doc("media", "synced.md")))
	_ = w.Close()
	ts := httptest.NewServer(server.NewRouter())
	defer ts.Close()
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/media/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		return errCase("reserved-upload", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	want := filepath.Join(pathutils.MediaRoot(), "media", sub, "pic.png")
	defer os.RemoveAll(filepath.Join(pathutils.MediaRoot(), "media", sub))
	var gaps []string
	if _, err := os.Stat(want); err != nil || resp.StatusCode != http.StatusOK {
		gaps = append(gaps, fmt.Sprintf("upload from %s: status %d (%s), %s exists %v", doc("media", "synced.md"), resp.StatusCode, strings.TrimSpace(string(body)), want, err == nil))
	}
	return gapsCase("reserved-upload", "an upload from a doc in docs/media/ lands in media/media/", gaps)
}
