package editorstest

import (
	"fmt"
	"os"
	"path/filepath"
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
	if err := files.UpdateLinksForSingleFile(pathutils.ToWithPrefix(relPath)); err != nil {
		return errCase(name, err)
	}

	got, err := readFile(relPath)
	if err != nil {
		return errCase(name, err)
	}
	meta, err := files.MetaDataGet(relPath)
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
		Path:       pathutils.ToWithPrefix(relPath),
		Editor:     files.EditorTypeIndex,
		Collection: base,
	}); err != nil {
		return errCase(name, err)
	}

	edited := initial + "\n## Second\n\n- [other.md](other.md)\n"
	if err := writeFile(relPath, edited); err != nil {
		return errCase(name, err)
	}
	if err := files.UpdateLinksForSingleFile(pathutils.ToWithPrefix(relPath)); err != nil {
		return errCase(name, err)
	}

	got, err := readFile(relPath)
	if err != nil {
		return errCase(name, err)
	}
	meta, err := files.MetaDataGet(relPath)
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
		Path:       pathutils.ToWithPrefix(relPath),
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
	if err := files.UpdateLinksForSingleFile(pathutils.ToWithPrefix(relPath)); err != nil {
		return errCase(name, err)
	}

	// the real file-view path: compose + render via files.GetFileContent
	content, err := files.GetFileContent(pathutils.ToDocsPath(relPath))
	if err != nil {
		return errCase(name, err)
	}
	meta, err := files.MetaDataGet(relPath)
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

// caseBookUnknownEntryRoundTrip verifies book.Parse keeps a line it doesn't model as an
// EntryUnknown that ToMarkdown writes back verbatim - so a hand edit in a `.book` file
// survives an editor save instead of being silently dropped - and that ComposeEntries
// leaves it out of the composed document.
func caseBookUnknownEntryRoundTrip() test.CaseResult {
	name := "book-unknown-entry"
	const stray = "plain prose the editor does not model"

	entries := book.Parse("# Kept Title\n\n" + stray + "\n\n- [[note.md]]\n")
	roundTrip := book.ToMarkdown(entries)
	reparsed := book.Parse(roundTrip)
	composed := book.ComposeEntries(name, entries)

	success := len(entries) == 3 &&
		entries[1].Type == book.EntryUnknown && entries[1].Value == stray &&
		strings.Contains(roundTrip, stray) && len(reparsed) == len(entries) &&
		!strings.Contains(composed, stray)

	cr := test.CaseResult{
		Name:     name,
		Expected: "unrecognized .book line round-trips through Parse->ToMarkdown->Parse verbatim and is omitted from the composed document",
		Actual:   fmt.Sprintf("entries=%+v roundTrip=%q composed=%q", entries, roundTrip, composed),
		Success:  success,
	}
	if !success {
		cr.Error = "unknown book entry was not preserved on round-trip"
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

	composed, err := book.Compose(bookPath)
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
	if err := handler.SaveTable(relPath, 0, []string{"A", "B"}, [][]string{{"3", "4"}, {"5", "6"}}); err != nil {
		return errCase(name, err)
	}
	if err := files.UpdateLinksForSingleFile(pathutils.ToWithPrefix(relPath)); err != nil {
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
