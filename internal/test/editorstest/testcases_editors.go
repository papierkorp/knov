package editorstest

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"knov/internal/book"
	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/pathutils"
	"knov/internal/test"
)

// createEditSaveCase covers the raw-content editors (codemirror): write
// initial content + metadata, overwrite with edited content, verify both persisted.
func createEditSaveCase(name, relPath string, editor files.EditorType, initial, edited string) test.CaseResult {
	if err := writeFile(relPath, initial); err != nil {
		return errCase(name, err)
	}
	if err := saveMetadata(relPath, editor); err != nil {
		return errCase(name, err)
	}
	if err := writeFile(relPath, edited); err != nil {
		return errCase(name, err)
	}
	if err := files.UpdateLinksForSingleFile(pathutils.GuessMeta(relPath)); err != nil {
		return errCase(name, err)
	}

	got, err := readFile(relPath)
	if err != nil {
		return errCase(name, err)
	}
	meta, err := files.MetaDataGet(pathutils.GuessMeta(relPath))
	if err != nil || meta == nil {
		return errCase(name, fmt.Errorf("metadata missing after save"))
	}

	success := got == edited && meta.Editor == editor
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("content=%q editor=%s", edited, editor),
		Actual:   fmt.Sprintf("content=%q editor=%s", got, meta.Editor),
		Success:  success,
	}
	if !success {
		cr.Error = "content or editor type mismatch after create+edit+save"
	}
	return cr
}

func caseCodeMirrorCreateEditSave() test.CaseResult {
	return createEditSaveCase("codemirror", testPath("codemirror.md"), files.EditorTypeCodeMirror,
		"# CodeMirror initial\n", "# CodeMirror edited\n")
}

// caseFilterCreateEditSave saves a filter config, resaves it with different criteria, and
// verifies the read-back config reflects the edit (filter.SaveFilterConfig/GetFilterConfig
// are the direct, non-HTTP path handleAPISaveFilterEditor -> handleAPIFilterSave ends up using).
func caseFilterCreateEditSave() test.CaseResult {
	name := "filter"
	id := testPath("edtest-filter")

	initial := &filter.Config{
		Criteria: []filter.Criteria{{Metadata: "tags", Operator: "contains", Value: "edtest-a", Action: "include"}},
		Logic:    "and",
	}
	if err := filter.SaveFilterConfig(initial, id); err != nil {
		return errCase(name, err)
	}

	edited := &filter.Config{
		Criteria: []filter.Criteria{{Metadata: "tags", Operator: "contains", Value: "edtest-b", Action: "include"}},
		Logic:    "and",
	}
	if err := filter.SaveFilterConfig(edited, id); err != nil {
		return errCase(name, err)
	}

	got, err := filter.GetFilterConfig(id)
	if err != nil || got == nil {
		return errCase(name, fmt.Errorf("failed to read back filter config"))
	}

	success := len(got.Criteria) == 1 && got.Criteria[0].Value == "edtest-b"
	cr := test.CaseResult{
		Name:     name,
		Expected: "criteria value edtest-b after edit",
		Actual:   fmt.Sprintf("criteria=%v", got.Criteria),
		Success:  success,
	}
	if !success {
		cr.Error = "filter config not updated correctly"
	}
	return cr
}

// caseListCreateEditSave mirrors handleAPISaveListEditor (list/todo editor merged, mode="list"):
// convert list items to markdown (render.ConvertListItemsToMarkdown just joins "- "+content+"\n"
// per item, replicated directly here rather than importing internal/server/render for one line of
// logic), write, save metadata tagged EditorTypeList.
func caseListCreateEditSave() test.CaseResult {
	name := "list"
	relPath := testPath("list") + configmanager.ExtensionForEditor("list")

	initial := "- first item\n- second item\n"
	if err := writeFile(relPath, initial); err != nil {
		return errCase(name, err)
	}
	if err := saveMetadata(relPath, files.EditorTypeList); err != nil {
		return errCase(name, err)
	}

	edited := "- first item\n- second item\n- third item\n"
	if err := writeFile(relPath, edited); err != nil {
		return errCase(name, err)
	}

	got, err := readFile(relPath)
	if err != nil {
		return errCase(name, err)
	}

	success := strings.Contains(got, "first item") && strings.Contains(got, "third item")
	cr := test.CaseResult{
		Name:     name,
		Expected: "markdown list containing first, second and third item",
		Actual:   got,
		Success:  success,
	}
	if !success {
		cr.Error = "list content missing expected items after edit"
	}
	return cr
}

// caseTodoCreateEditSave mirrors handleAPISaveListEditor (mode="todo"): GFM checkbox markdown
// (state prefixes "[ ] "/"[X] " match render.stateToMarkdown's open/done cases, replicated
// directly here to avoid importing internal/server/render - see caseListCreateEditSave),
// edited to flip a task's state.
func caseTodoCreateEditSave() test.CaseResult {
	name := "todo"
	relPath := testPath("todo") + configmanager.ExtensionForEditor("todo")

	initial := "- [ ] task one\n"
	if err := writeFile(relPath, initial); err != nil {
		return errCase(name, err)
	}
	if err := saveMetadata(relPath, files.EditorTypeTodo); err != nil {
		return errCase(name, err)
	}

	edited := "- [X] task one\n"
	if err := writeFile(relPath, edited); err != nil {
		return errCase(name, err)
	}

	got, err := readFile(relPath)
	if err != nil {
		return errCase(name, err)
	}

	success := strings.Contains(got, "[X] task one")
	cr := test.CaseResult{
		Name:     name,
		Expected: "checkbox for 'task one' marked [X] (done) after edit",
		Actual:   got,
		Success:  success,
	}
	if !success {
		cr.Error = "todo checkbox state not updated after edit"
	}
	return cr
}

// caseIndexCreateEditSave mirrors handleAPISaveIndexEditor: markdown built from index
// entries, metadata with a derived collection name, edited to add a second section.
func caseIndexCreateEditSave() test.CaseResult {
	name := "index"
	base := testPath("myindex")
	relPath := base + configmanager.ExtensionForEditor("index")

	initial := "## Section\n\n- [target.md](target.md)\n"
	if err := writeFile(relPath, initial); err != nil {
		return errCase(name, err)
	}
	if err := test.SeedMetadata(&files.Metadata{
		Path:       pathutils.GuessMeta(relPath),
		Editor:     files.EditorTypeIndex,
		Collection: base,
	}); err != nil {
		return errCase(name, err)
	}

	edited := initial + "\n## Second\n\n- [other.md](other.md)\n"
	if err := writeFile(relPath, edited); err != nil {
		return errCase(name, err)
	}
	if err := files.UpdateLinksForSingleFile(pathutils.GuessMeta(relPath)); err != nil {
		return errCase(name, err)
	}

	got, err := readFile(relPath)
	if err != nil {
		return errCase(name, err)
	}
	meta, err := files.MetaDataGet(pathutils.GuessMeta(relPath))
	if err != nil || meta == nil {
		return errCase(name, fmt.Errorf("metadata missing after save"))
	}

	success := strings.Contains(got, "Second") && meta.Editor == files.EditorTypeIndex
	cr := test.CaseResult{
		Name:     name,
		Expected: "content contains 'Second' section, editor=index-editor",
		Actual:   fmt.Sprintf("editor=%s content=%q", meta.Editor, got),
		Success:  success,
	}
	if !success {
		cr.Error = "index content or editor type mismatch after edit"
	}
	return cr
}

// caseBookCreateEditSave exercises the internal/book round-trip that
// handleAPISaveIndexEditor (mode=book) drives (build entries -> ToMarkdown -> save,
// reload with Parse, append an entry, re-save) and then the real file-view path:
// files.GetFileContent detects the book, composes its entries into one document and
// renders it, with the referenced section/whole-file bodies inlined and no
// per-section edit buttons (the composed doc has no source file). One section entry
// targets its heading by visible text ("...#My Section"), exercising book.Compose's
// slugification to the id the renderer generates rather than a pre-slugified anchor.
func caseBookCreateEditSave() test.CaseResult {
	name := "book"
	base := testPath("mybook")
	relPath := base + configmanager.ExtensionForEditor("book")

	// sources the book composes from
	target := testPath("book-target.md")
	appendix := testPath("book-appendix.md")
	if err := writeFile(target, "# Target\n\n## Details\n\ndetail body\n\n## My Section\n\nmy section body\n"); err != nil {
		return errCase(name, err)
	}
	if err := writeFile(appendix, "# Appendix\n\nappendix body\n"); err != nil {
		return errCase(name, err)
	}

	// build + save the book the way the save handler does
	entries := []book.Entry{
		{Type: book.EntryTitle, Value: "Intro"},
		{Type: book.EntryFile, Value: target + "#details", IncludeSubheaders: true},
		{Type: book.EntryFile, Value: target + "#My Section"}, // anchor by visible heading text, not a slug
		{Type: book.EntrySeparator},
	}
	if err := writeFile(relPath, book.ToMarkdown(entries)); err != nil {
		return errCase(name, err)
	}
	if err := test.SeedMetadata(&files.Metadata{
		Path:       pathutils.GuessMeta(relPath),
		Editor:     files.EditorTypeBook,
		Collection: base,
	}); err != nil {
		return errCase(name, err)
	}

	// reload (editor re-open) and assert the round-trip before editing further
	saved, err := readFile(relPath)
	if err != nil {
		return errCase(name, err)
	}
	reloaded := book.Parse(saved)
	if !(len(reloaded) == 4 &&
		reloaded[0].Type == book.EntryTitle && reloaded[0].Value == "Intro" &&
		reloaded[1].Type == book.EntryFile && reloaded[1].IncludeSubheaders &&
		reloaded[2].Type == book.EntryFile && reloaded[2].Value == target+"#My Section" && !reloaded[2].IncludeSubheaders &&
		reloaded[3].Type == book.EntrySeparator) {
		return errCase(name, fmt.Errorf("entries did not round-trip through Parse: %+v", reloaded))
	}

	// append an entry, re-save
	reloaded = append(reloaded, book.Entry{Type: book.EntryFile, Value: appendix})
	if err := writeFile(relPath, book.ToMarkdown(reloaded)); err != nil {
		return errCase(name, err)
	}
	if err := files.UpdateLinksForSingleFile(pathutils.GuessMeta(relPath)); err != nil {
		return errCase(name, err)
	}

	// the real file-view path: compose + render via files.GetFileContent
	content, err := files.GetFileContent(pathutils.GuessMeta(relPath))
	if err != nil {
		return errCase(name, err)
	}
	meta, err := files.MetaDataGet(pathutils.GuessMeta(relPath))
	if err != nil || meta == nil {
		return errCase(name, fmt.Errorf("metadata missing after save"))
	}

	success := strings.Contains(content.HTML, "Intro") &&
		strings.Contains(content.HTML, "detail body") &&
		strings.Contains(content.HTML, "my section body") && // reached via the visible-text "#My Section" anchor
		strings.Contains(content.HTML, "appendix body") &&
		!strings.Contains(content.HTML, "header-edit-btn") &&
		meta.Editor == files.EditorTypeBook
	cr := test.CaseResult{
		Name:     name,
		Expected: "book round-trips through Parse, file view composes intro + section body + visible-text section body + appendix body with no section edit buttons, editor=book-editor",
		Actual:   fmt.Sprintf("editor=%s html=%q", meta.Editor, content.HTML),
		Success:  success,
	}
	if !success {
		cr.Error = "book round-trip or composed file-view output mismatch"
	}
	return cr
}

// caseBookUnknownEntryRoundTrip verifies book.Parse coalesces a run of lines it doesn't
// model into one EntryUnknown that ToMarkdown writes back verbatim - so a hand edit in a
// `.book` file survives an editor save instead of being silently dropped - that
// ComposeEntries emits it verbatim into the composed document, that an interior blank line
// (a paragraph break) is preserved, that leading indentation (a nested list) is kept, that
// an indented line is never promoted to a flush-left entry, that a CRLF hand edit is
// normalized to LF, and that a fenced block is kept rather than dropped.
func caseBookUnknownEntryRoundTrip() test.CaseResult {
	name := "book-unknown-entry"
	const stray = "plain prose the editor does not model"

	entries := book.Parse("# Kept Title\n\n" + stray + "\n\n- [[note.md]]\n")
	roundTrip := book.ToMarkdown(entries)
	reparsed := book.Parse(roundTrip)
	composed := book.ComposeEntries(name, entries)

	// a hand-written block with an interior blank line: the blank must survive so the two
	// paragraphs are not merged into one on round-trip or compose
	const block = "| a | b |\n| - | - |\n\nafter the table"
	blockEntries := book.Parse("- [[x.md]]\n\n" + block + "\n")

	// a fenced block inside a hand edit is kept verbatim (fences and all), not dropped
	const fenced = "```\nplain fenced\n```"
	fencedEntries := book.Parse("- [[x.md]]\n\n" + fenced + "\n")

	// a hand-written nested list: leading indentation must survive verbatim, not be flattened
	const nested = "- outer\n  - inner\n    - deep"
	nestedEntries := book.Parse("- [[x.md]]\n\n" + nested + "\n")

	// an indented "- [[ref]]" / "## heading" is hand-written nesting: it stays verbatim,
	// never promoted to a flush-left reference/title (which would flatten the indentation)
	const indented = "  - [[child.md]]\n  ## indented heading\n  more"
	indentedEntries := book.Parse("- [[top.md]]\n\n" + indented + "\n")

	// a CRLF hand edit (Windows editor / textarea POST) is normalized to LF - no stray \r
	// survives into the verbatim block
	crlf := book.Parse("- [[x.md]]\r\n\r\nline one\r\nline two\r\n")

	// an empty-Value unknown (only reachable via a hand-crafted POST) is dropped by
	// ToMarkdown and ComposeEntries, never emitted as a stray blank line
	strayBlank := []book.Entry{{Type: book.EntryUnknown}, {Type: book.EntryUnknown, Value: "x"}}

	success := len(entries) == 3 &&
		entries[1].Type == book.EntryUnknown && entries[1].Value == stray &&
		strings.Contains(roundTrip, stray) && len(reparsed) == len(entries) &&
		strings.Contains(composed, stray) &&
		strings.Contains(book.ToMarkdown(blockEntries), block) &&
		strings.Contains(book.ComposeEntries(name, blockEntries), block) &&
		len(fencedEntries) == 2 && fencedEntries[1].Value == fenced &&
		strings.Contains(book.ToMarkdown(fencedEntries), fenced) &&
		len(nestedEntries) == 2 && nestedEntries[1].Value == nested &&
		strings.Contains(book.ComposeEntries(name, nestedEntries), nested) &&
		len(indentedEntries) == 2 && indentedEntries[1].Type == book.EntryUnknown &&
		indentedEntries[1].Value == indented &&
		len(crlf) == 2 && crlf[1].Value == "line one\nline two" &&
		book.ToMarkdown(strayBlank) == "x\n" &&
		book.ComposeEntries(name, strayBlank) == "x"

	cr := test.CaseResult{
		Name:     name,
		Expected: "unrecognized .book line round-trips through Parse->ToMarkdown->Parse verbatim and is emitted verbatim into the composed document",
		Actual:   fmt.Sprintf("entries=%+v roundTrip=%q composed=%q", entries, roundTrip, composed),
		Success:  success,
	}
	if !success {
		cr.Error = "unknown book entry was not preserved on round-trip"
	}
	return cr
}

// caseBookTitleLevelRoundTrip verifies book.Parse keeps the "#" depth of a hand-written
// "## Sub Heading" as Entry.Level and that ToMarkdown / ComposeEntries write it back with
// the same number of "#" rather than collapsing every title to a single "#" - that a line
// that is not a real ATX heading (7+ "#", or "#" with no space) is kept verbatim rather
// than promoted to a title - and that an out-of-range or missing Level is clamped to [1,6]
// so a bogus form value can't be emitted.
func caseBookTitleLevelRoundTrip() test.CaseResult {
	name := "book-title-level"

	entries := book.Parse("## Sub Heading\n\n- [[note.md]]\n")
	roundTrip := book.ToMarkdown(entries)
	composed := book.ComposeEntries(name, entries)

	// not headings: 7+ "#" and "#tag" (no space) stay verbatim as an unknown block
	deep := book.Parse("####### Too Deep\n\n#tag\n")

	// clamp: a missing (0), negative or huge Level falls back to a single "#"
	clamp := book.ToMarkdown([]book.Entry{
		{Type: book.EntryTitle, Value: "Zero", Level: 0},
		{Type: book.EntryTitle, Value: "Neg", Level: -3},
		{Type: book.EntryTitle, Value: "Huge", Level: 999},
	})

	success := len(entries) == 2 &&
		entries[0].Type == book.EntryTitle && entries[0].Level == 2 &&
		strings.Contains(roundTrip, "\n## Sub Heading\n") &&
		strings.HasPrefix(composed, "## Sub Heading\n\n") &&
		len(deep) == 1 && deep[0].Type == book.EntryUnknown &&
		deep[0].Value == "####### Too Deep\n\n#tag" &&
		strings.Contains(clamp, "\n# Zero\n") &&
		strings.Contains(clamp, "\n# Neg\n") &&
		strings.Contains(clamp, "\n# Huge\n")

	cr := test.CaseResult{
		Name:     name,
		Expected: "hand-written '## Sub Heading' round-trips through Parse->ToMarkdown with its heading depth kept as Level 2",
		Actual:   fmt.Sprintf("entries=%+v roundTrip=%q composed=%q", entries, roundTrip, composed),
		Success:  success,
	}
	if !success {
		cr.Error = "book title heading level was not preserved on round-trip"
	}
	return cr
}

// caseBookPathContainment verifies a `.book` entry can't escape the docs root: an absolute
// ("/etc/passwd") or "../"-traversal target must resolve inside docs (where it does not
// exist) and show the "could not include" marker, never the outside file's content. Guards
// against a future refactor swapping book.Compose's pathutils.ToDocsPath (which clamps) for
// an unclamped resolver.
func caseBookPathContainment() test.CaseResult {
	name := "book-path-containment"

	// a secret sitting just outside the docs root
	secret := filepath.Join(pathutils.DocsRoot(), "..", "book-outside-secret.txt")
	if err := os.WriteFile(secret, []byte("TOP-SECRET-OUTSIDE-DOCS"), 0644); err != nil {
		return errCase(name, err)
	}
	defer os.Remove(secret)

	valid := testPath("containment-valid.md")
	if err := writeFile(valid, "# Valid\n\nvalid body\n"); err != nil {
		return errCase(name, err)
	}

	bookPath := testPath("containment.book")
	entries := []book.Entry{
		{Type: book.EntryFile, Value: valid},
		{Type: book.EntryFile, Value: "../book-outside-secret.txt"},
		{Type: book.EntryFile, Value: "/etc/passwd"},
		{Type: book.EntryFile, Value: "../../../../etc/shadow"},
	}
	if err := writeFile(bookPath, book.ToMarkdown(entries)); err != nil {
		return errCase(name, err)
	}

	composed, err := book.Compose(pathutils.DocsPath(bookPath))
	if err != nil {
		return errCase(name, err)
	}

	docsRoot := pathutils.DocsRoot()
	leaked := strings.Contains(composed, "TOP-SECRET-OUTSIDE-DOCS") || strings.Contains(composed, "root:")
	clamped := strings.HasPrefix(pathutils.ToDocsPath("/etc/passwd"), docsRoot) &&
		strings.HasPrefix(pathutils.ToDocsPath("../../book-outside-secret.txt"), docsRoot)
	success := !leaked && clamped &&
		strings.Contains(composed, "valid body") &&
		strings.Contains(composed, "could not include")

	cr := test.CaseResult{
		Name:     name,
		Expected: "outside-docs and absolute .book entries clamp into docs, never leak external content",
		Actual:   fmt.Sprintf("leaked=%t clamped=%t composed=%q", leaked, clamped, composed),
		Success:  success,
	}
	if !success {
		cr.Error = "book path containment failed"
	}
	return cr
}

// caseBookFilterEntry verifies a `.book` "<!-- filter: id -->" entry composes to the
// matching files of the real saved filter (filter.SavedFilterPaths via book.FilterResolver):
// tagged text files are inlined, a tagged binary and the book itself are skipped silently,
// and a missing filter shows the "could not include" marker.
func caseBookFilterEntry() test.CaseResult {
	name := "book-filter-entry"
	tag := "edtest-bookfilter"

	filterID := testPath("bookfilter")
	if err := filter.SaveFilterConfig(&filter.Config{
		Criteria: []filter.Criteria{{Metadata: "tags", Operator: "contains", Value: tag, Action: "include"}},
		Logic:    "and",
	}, filterID); err != nil {
		return errCase(name, err)
	}
	defer filter.DeleteFilterConfig(filterID)

	bookPath := testPath("bookfilter.book")
	seeds := map[string]string{
		testPath("bookfilter-a.md"):   "# A\n\nfilter body a\n",
		testPath("bookfilter-b.txt"):  "filter body b\n",
		testPath("bookfilter-c.png"):  "PNG-GARBAGE",
		testPath("bookfilter-off.md"): "untagged body\n",
		bookPath: book.ToMarkdown([]book.Entry{
			{Type: book.EntryFilter, Value: filterID},
			{Type: book.EntryFilter, Value: testPath("bookfilter-missing")},
		}),
	}
	for p, content := range seeds {
		if err := writeFile(p, content); err != nil {
			return errCase(name, err)
		}
		meta := &files.Metadata{Path: pathutils.GuessMeta(p), Tags: []string{tag}}
		if strings.HasSuffix(p, "-off.md") {
			meta.Tags = nil
		}
		if p == bookPath {
			meta.Editor = files.EditorTypeBook
		}
		if err := test.SeedMetadata(meta); err != nil {
			return errCase(name, err)
		}
	}

	composed, err := book.Compose(pathutils.DocsPath(bookPath))
	if err != nil {
		return errCase(name, err)
	}

	success := strings.Contains(composed, "filter body a") &&
		strings.Contains(composed, "filter body b") &&
		!strings.Contains(composed, "untagged body") &&
		!strings.Contains(composed, "PNG-GARBAGE") &&
		!strings.Contains(composed, "bookfilter-c.png") &&
		!strings.Contains(composed, "<!-- filter:") &&
		strings.Count(composed, "could not include") == 1 &&
		strings.Contains(composed, "bookfilter-missing")

	cr := test.CaseResult{
		Name:     name,
		Expected: "tagged text files inlined, binary/self/untagged skipped, one marker for the missing filter",
		Actual:   fmt.Sprintf("composed=%q", composed),
		Success:  success,
	}
	if !success {
		cr.Error = "book filter entry did not compose as expected"
	}
	return cr
}

// caseTableCreateEditSave mirrors handleAPITableEditorSave: build a markdown table, then
// call the real MarkdownContentHandler.SaveTable to edit it in place (also exercises the
// "table-save" operation from the build-order todo, same underlying call).
func caseTableCreateEditSave() test.CaseResult {
	name := "table"
	relPath := testPath("table.md")

	initial := "# Table doc\n\n| A | B |\n| --- | --- |\n| 1 | 2 |\n"
	if err := writeFile(relPath, initial); err != nil {
		return errCase(name, err)
	}
	if err := saveMetadata(relPath, files.EditorTypeCodeMirror); err != nil {
		return errCase(name, err)
	}

	handler := contentHandler.GetHandler("markdown")
	if err := handler.SaveTable(pathutils.DocsPath(relPath), 0, []string{"A", "B"}, [][]string{{"3", "4"}, {"5", "6"}}, nil); err != nil {
		return errCase(name, err)
	}
	if err := files.UpdateLinksForSingleFile(pathutils.GuessMeta(relPath)); err != nil {
		return errCase(name, err)
	}

	got, err := readFile(relPath)
	if err != nil {
		return errCase(name, err)
	}

	success := strings.Contains(got, "3") && strings.Contains(got, "4") &&
		strings.Contains(got, "5") && strings.Contains(got, "6")
	cr := test.CaseResult{
		Name:     name,
		Expected: "table rows replaced with 3,4 / 5,6",
		Actual:   got,
		Success:  success,
	}
	if !success {
		cr.Error = "table content not updated after SaveTable"
	}
	return cr
}

// caseTableAlignRoundTrip exercises per-column alignment: parse a markdown table with
// mixed left/center/right columns, then save new alignment and confirm it survives
// a second extract (mirrors the ExtractTable/SaveTable contract used by the table editor API).
func caseTableAlignRoundTrip() test.CaseResult {
	name := "table-align"
	relPath := testPath("table_align.md")

	initial := "# Aligned table\n\n| A | B | C |\n| :--- | :---: | ---: |\n| 1 | 2 | 3 |\n"
	if err := writeFile(relPath, initial); err != nil {
		return errCase(name, err)
	}
	if err := saveMetadata(relPath, files.EditorTypeCodeMirror); err != nil {
		return errCase(name, err)
	}

	handler := contentHandler.GetHandler("markdown")
	_, _, aligns, err := handler.ExtractTable(pathutils.DocsPath(relPath), 0)
	if err != nil {
		return errCase(name, err)
	}
	wantInitial := []string{"left", "center", "right"}
	if !slices.Equal(aligns, wantInitial) {
		return errCase(name, fmt.Errorf("extracted aligns = %v, want %v", aligns, wantInitial))
	}

	newAligns := []string{"right", "left", "center"}
	if err := handler.SaveTable(pathutils.DocsPath(relPath), 0, []string{"A", "B", "C"}, [][]string{{"1", "2", "3"}}, newAligns); err != nil {
		return errCase(name, err)
	}

	_, _, gotAligns, err := handler.ExtractTable(pathutils.DocsPath(relPath), 0)
	if err != nil {
		return errCase(name, err)
	}

	success := slices.Equal(gotAligns, newAligns)
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("aligns %v persisted and re-extracted", newAligns),
		Actual:   fmt.Sprintf("%v", gotAligns),
		Success:  success,
	}
	if !success {
		cr.Error = "alignment did not round-trip through SaveTable/ExtractTable"
	}
	return cr
}
