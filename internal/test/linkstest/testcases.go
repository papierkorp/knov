package linkstest

import (
	"bytes"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/server"
	"knov/internal/test"
)

// caseUpload uploads into a doc in a folder named after each corpus name, from the doc the upload
// api reads out of the context_path uploadMediaBlob sends (the edit page's location.pathname),
// and inserts the link it returns - the media file has to mirror the doc's folder. The media page
// is requested the way a browser encodes the url: it keeps ( ) ' raw where go escapes them, so the
// request has a RawPath and the route must not read chi's still-encoded wildcard.
func caseUpload() test.CaseResult {
	ts := httptest.NewServer(server.NewRouter())
	defer ts.Close()
	browserURL := strings.NewReplacer("%28", "(", "%29", ")", "%27", "'")

	var gaps []string
	for _, n := range names {
		folder := testDir + "/upload/" + strings.TrimSuffix(n, ".md")
		doc := folder + "/doc.md"
		if err := saveDoc(doc, "# doc\n"); err != nil {
			return errCase("links-upload", err)
		}
		file, header, err := multipartFile("pic.png", pngMagic)
		if err != nil {
			return errCase("links-upload", err)
		}
		res, err := files.UploadMedia(file, header, pathutils.FileFromURL(pathutils.ToFileEditURL(doc)))
		if err != nil {
			gaps = append(gaps, fmt.Sprintf("upload into %q: %v", folder, err))
			continue
		}
		want := "media/" + folder + "/pic.png"
		if "media/"+res.Path != want {
			gaps = append(gaps, fmt.Sprintf("%q: uploaded to %q", want, "media/"+res.Path))
		}
		resp, err := ts.Client().Get(ts.URL + browserURL.Replace(pathutils.ToMediaURL(res.Path)))
		if err != nil {
			return errCase("links-upload", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			gaps = append(gaps, fmt.Sprintf("%q: media page status %d", want, resp.StatusCode))
		}
		if err := saveDoc(doc, res.Link+"\n"); err != nil {
			return errCase("links-upload", err)
		}
		gaps = append(gaps, linkGaps(doc, []string{want})...)
	}
	return gapsCase("links-upload", "an upload from a doc in every special-char folder lands in its media mirror, is served and the inserted link reads back as it", gaps)
}

// caseRepairOldUpload rebuilds what the old upload left behind for a doc in every special-char
// folder: media stored under the folder as the browser encoded it (location.pathname, `( ) '`
// kept raw) and the raw `media/<path>` link, which now reads as the missing decoded folder. The
// broken-links check has to suggest the media file at the literal link path, and its repair has
// to make the link read back as that file.
func caseRepairOldUpload() test.CaseResult {
	browserURL := strings.NewReplacer("%28", "(", "%29", ")", "%27", "'")
	broken, err := func() (map[string]string, error) {
		links := map[string]string{}
		for _, n := range names {
			folder := testDir + "/repair/" + strings.TrimSuffix(n, ".md")
			literal := strings.TrimPrefix(browserURL.Replace(pathutils.ToRouteURL("/", folder)), "/") + "/pic.png"
			if literal == folder+"/pic.png" {
				continue
			}
			full := pathutils.ToMediaPath(literal)
			if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
				return nil, err
			}
			if err := contentStorage.WriteFile(full, pngMagic, 0644); err != nil {
				return nil, err
			}
			if err := files.MetaDataSync("media/" + literal); err != nil {
				return nil, err
			}
			if err := saveDoc(folder+"/doc.md", "![pic.png](media/"+literal+")\n"); err != nil {
				return nil, err
			}
			links[pathutils.ToWithPrefix(folder+"/doc.md")] = "media/" + literal
		}
		return links, nil
	}()
	if err != nil {
		return errCase("links-repair-old-upload", err)
	}
	files.RefreshCaches()
	found, err := files.FindBrokenLinks()
	if err != nil {
		return errCase("links-repair-old-upload", err)
	}
	var gaps []string
	for _, bl := range found {
		want, ok := broken[bl.SourceFile]
		if !ok {
			continue
		}
		delete(broken, bl.SourceFile)
		if bl.Suggested != want {
			gaps = append(gaps, fmt.Sprintf("%q: suggested %q for %q", want, bl.Suggested, bl.Target))
			continue
		}
		if ok, err := files.RepairBrokenLink(bl.SourceFile, bl.Target, bl.Suggested); err != nil || !ok {
			gaps = append(gaps, fmt.Sprintf("%q: repair failed (%v)", want, err))
			continue
		}
		gaps = append(gaps, linkGaps(strings.TrimPrefix(bl.SourceFile, "docs/"), []string{want})...)
	}
	for src, want := range broken {
		gaps = append(gaps, fmt.Sprintf("%q in %q: not reported as broken", want, src))
	}
	return gapsCase("links-repair-old-upload", "every link the old upload inserted into a special-char folder is suggested and repaired to its media file", gaps)
}

// caseRename renames a linked file to every corpus name and away again - the links rename wrote
// (markdown, wiki, extensionless wiki, /files/ url, html) have to read back as the new file. A
// name the filename policy rejects can't be renamed to (see pathutils.TestCheckMovedName), the file
// is written directly (like git sync) and only renamed away.
func caseRename() test.CaseResult {
	var gaps []string
	for i, n := range names {
		dir := fmt.Sprintf("%s/rename/c%02d", testDir, i)
		old, renamed, back := dir+"/old.md", dir+"/"+n, dir+"/back.md"
		moves := [][2]string{{old, renamed}, {renamed, back}}
		if errors.Is(pathutils.CheckNewDocsPath(renamed), pathutils.ErrInvalidName) {
			old, moves = renamed, moves[1:]
		}
		if err := saveDoc(old, "# old\n"); err != nil {
			return errCase("links-rename", err)
		}
		srcs, err := saveForms(dir, map[string]string{
			"markdown":    parser.Link{Kind: parser.LinkMarkdown, Text: "x", Path: old}.String(),
			"wiki":        parser.Link{Kind: parser.LinkWiki, Path: old}.String(),
			"wiki no ext": parser.Link{Kind: parser.LinkWiki, Path: strings.TrimSuffix(old, ".md")}.String(),
			"file url":    "[x](" + pathutils.ToFileURL(old) + ")",
			"html":        `<a href="` + pathutils.ToFileURL(old) + `">x</a>`,
		})
		if err != nil {
			return errCase("links-rename", err)
		}
		for _, mv := range moves {
			if err := files.MoveFileNoRefresh(logging.KeyApp, mv[0], mv[1]); err != nil {
				gaps = append(gaps, fmt.Sprintf("rename %q -> %q: %v", mv[0], mv[1], err))
				break
			}
			for form, src := range srcs {
				for _, g := range formGaps(form, src, []string{pathutils.ToWithPrefix(mv[1])}) {
					gaps = append(gaps, fmt.Sprintf("%s link after rename to %q: %s", form, mv[1], g))
				}
			}
		}
	}
	files.RefreshCaches()
	return gapsCase("links-rename", "links renamed to and away from every special-char name read back as the renamed file", gaps)
}

// caseRelocate puts a misplaced image named after each corpus name into the docs folder, links it
// (markdown, wiki, html /files/ url, doc-relative markdown) and relocates it into the media folder -
// the rewritten links have to read back as the moved file.
func caseRelocate() test.CaseResult {
	var selected []string
	var srcs []map[string]string
	for i, n := range names {
		dir := fmt.Sprintf("%s/relocate/c%02d", testDir, i)
		img := dir + "/" + imgName(n)
		if err := writeDoc(img, pngMagic); err != nil {
			return errCase("links-relocate", err)
		}
		forms, err := saveForms(dir, map[string]string{
			"markdown":          parser.Link{Kind: parser.LinkMarkdown, Image: true, Text: "x", Path: img}.String(),
			"wiki":              parser.Link{Kind: parser.LinkWiki, Path: img}.String(),
			"html":              `<img src="` + pathutils.ToFileURL(img) + `">`,
			"markdown relative": parser.Link{Kind: parser.LinkMarkdown, Image: true, Text: "y", Path: imgName(n)}.String(),
		})
		if err != nil {
			return errCase("links-relocate", err)
		}
		selected = append(selected, img)
		srcs = append(srcs, forms)
	}
	files.RefreshCaches()
	res, err := files.RelocateMisplacedMedia(logging.KeyApp, selected)
	if err != nil {
		return errCase("links-relocate", err)
	}
	var gaps []string
	if res.Moved != len(selected) || res.Failed != 0 {
		gaps = append(gaps, fmt.Sprintf("moved %d of %d, %d failed", res.Moved, len(selected), res.Failed))
	}
	for i, forms := range srcs {
		for form, src := range forms {
			for _, g := range formGaps(form, src, []string{"media/" + selected[i]}) {
				gaps = append(gaps, form+" link: "+g)
			}
		}
	}
	return gapsCase("links-relocate", "every special-char misplaced image is moved to its media mirror and all its links read back as the moved file", gaps)
}

// caseFilterIndex saves a filter selecting every target doc - its generated index has to link
// each one.
func caseFilterIndex() test.CaseResult {
	const id = "links-tests"
	cfg := &filter.Config{
		Criteria: []filter.Criteria{{Metadata: "folders", Operator: "equals", Value: targetsFolder, Action: "include"}},
		Logic:    "and",
		Display:  "list",
	}
	if err := filter.SaveFilterConfig(cfg, id); err != nil {
		return errCase("links-filter-index", err)
	}
	return gapsCase("links-filter-index", "the generated filter index links every special-char doc", linkGaps(filter.FilterIndexPath(id), allTargets()))
}

// caseBookEditor saves a book with every target doc as entry through the real book editor api
// (the plain path as typed or picked) - the book has to link and include each one. A typed path
// is trimmed, so a name with a trailing space can't be an entry - the app never creates one
// (filename policy), only git sync or a manual copy can.
func caseBookEditor() test.CaseResult {
	ts := httptest.NewServer(server.NewRouter())
	defer ts.Close()

	book := testDir + "/links.book"
	form := url.Values{"filepath": {book}}
	var entries []int
	for i := range names {
		if strings.TrimSpace(target(i)) != target(i) {
			continue
		}
		n := len(entries)
		entries = append(entries, i)
		form.Set(fmt.Sprintf("entries[%d][type]", n), "file")
		form.Set(fmt.Sprintf("entries[%d][value]", n), target(i))
	}
	resp, err := ts.Client().PostForm(ts.URL+"/api/editor/bookeditor", form)
	if err != nil {
		return errCase("links-book-editor", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errCase("links-book-editor", fmt.Errorf("save status %d", resp.StatusCode))
	}

	want := make([]string, len(entries))
	for n, i := range entries {
		want[n] = pathutils.ToWithPrefix(target(i))
	}
	gaps := metadataGaps(book, want)
	fc, err := files.GetFileContent(pathutils.ToDocsPath(book))
	if err != nil {
		return errCase("links-book-editor", err)
	}
	for _, i := range entries {
		if !strings.Contains(fc.HTML, marker(i)) {
			gaps = append(gaps, fmt.Sprintf("%q: not included in the composed book", target(i)))
		}
	}
	return gapsCase("links-book-editor", "a book of every special-char doc links and includes each one", gaps)
}

// formGaps checks the links of a form's doc - an "html" one only through link metadata, raw html
// is never rendered (goldmark without WithUnsafe).
func formGaps(form, src string, want []string) []string {
	if form == "html" {
		return metadataGaps(src, want)
	}
	return linkGaps(src, want)
}

// saveForms saves each link form into its own doc in dir, so a broken form can't hide behind a
// working one, and returns the doc per form.
func saveForms(dir string, forms map[string]string) (map[string]string, error) {
	srcs := make(map[string]string, len(forms))
	for form, link := range forms {
		srcs[form] = dir + "/src-" + strings.ReplaceAll(form, " ", "-") + ".md"
		if err := saveDoc(srcs[form], link+"\n"); err != nil {
			return nil, err
		}
	}
	return srcs, nil
}

// allTargets returns the metadata path of every seeded doc.
func allTargets() []string {
	want := make([]string, len(names))
	for i := range names {
		want[i] = pathutils.ToWithPrefix(target(i))
	}
	return want
}

// multipartFile builds the multipart.File/FileHeader pair the upload handler passes on.
func multipartFile(filename string, content []byte) (multipart.File, *multipart.FileHeader, error) {
	buf := new(bytes.Buffer)
	writer := multipart.NewWriter(buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, nil, err
	}
	if _, err := part.Write(content); err != nil {
		return nil, nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, nil, err
	}
	form, err := multipart.NewReader(buf, writer.Boundary()).ReadForm(int64(len(content)) + 1024)
	if err != nil {
		return nil, nil, err
	}
	header := form.File["file"][0]
	file, err := header.Open()
	return file, header, err
}
