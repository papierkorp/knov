package editorstest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"knov/internal/contentHandler"
	"knov/internal/dokuwikiconverter"
	"knov/internal/files"
	"knov/internal/parser"
	"knov/internal/server"
	"knov/internal/test"
	"knov/internal/utils"
)

// caseSectionSave mirrors handleAPISaveSectionEditor: replace the body of a single markdown
// section by its heading-derived ID via MarkdownContentHandler.SaveSection.
func caseSectionSave() test.CaseResult {
	name := "section-save"
	relPath := testPath("sections.md")

	initial := "# Doc\n\n## First Section\n\noriginal content\n\n## Second Section\n\nother content\n"
	if err := writeFile(relPath, initial); err != nil {
		return errCase(name, err)
	}
	if err := saveMetadata(relPath, files.EditorTypeCodeMirror); err != nil {
		return errCase(name, err)
	}

	sectionID := utils.GenerateID("First Section", map[string]int{})
	handler := contentHandler.GetHandler("markdown")
	if err := handler.SaveSection(relPath, sectionID, "updated content"); err != nil {
		return errCase(name, err)
	}

	got, err := readFile(relPath)
	if err != nil {
		return errCase(name, err)
	}

	success := strings.Contains(got, "updated content") &&
		!strings.Contains(got, "original content") &&
		strings.Contains(got, "other content")
	cr := test.CaseResult{
		Name:     name,
		Expected: "First Section body replaced, Second Section untouched",
		Actual:   got,
		Success:  success,
	}
	if !success {
		cr.Error = "section content not replaced correctly"
	}
	return cr
}

// caseTodoToggle mirrors handleAPIToggleTodoState: cycle a checkbox's state via
// parser.CycleTodoStateAtLine (open -> done).
func caseTodoToggle() test.CaseResult {
	name := "todo-toggle"
	relPath := testPath("toggle.md")

	if err := writeFile(relPath, "- [ ] task one\n"); err != nil {
		return errCase(name, err)
	}
	if err := saveMetadata(relPath, files.EditorTypeCodeMirror); err != nil {
		return errCase(name, err)
	}

	content, err := readFile(relPath)
	if err != nil {
		return errCase(name, err)
	}
	updated, _, err := parser.CycleTodoStateAtLine([]byte(content), 0)
	if err != nil {
		return errCase(name, err)
	}
	if err := writeFile(relPath, string(updated)); err != nil {
		return errCase(name, err)
	}

	got, err := readFile(relPath)
	if err != nil {
		return errCase(name, err)
	}

	success := strings.Contains(got, "[X] task one")
	cr := test.CaseResult{
		Name:     name,
		Expected: "- [X] task one",
		Actual:   got,
		Success:  success,
	}
	if !success {
		cr.Error = "checkbox state did not cycle from open to done"
	}
	return cr
}

// caseTodoClearDate mirrors handleAPIClearTodoDate: strip a stamped date via
// parser.ClearTodoDateAtLine without touching the checkbox state.
func caseTodoClearDate() test.CaseResult {
	name := "todo-clear-date"
	relPath := testPath("cleardate.md")

	if err := writeFile(relPath, "- [X] task one (2026-01-01)\n"); err != nil {
		return errCase(name, err)
	}
	if err := saveMetadata(relPath, files.EditorTypeCodeMirror); err != nil {
		return errCase(name, err)
	}

	content, err := readFile(relPath)
	if err != nil {
		return errCase(name, err)
	}
	updated, err := parser.ClearTodoDateAtLine([]byte(content), 0)
	if err != nil {
		return errCase(name, err)
	}
	if err := writeFile(relPath, string(updated)); err != nil {
		return errCase(name, err)
	}

	got, err := readFile(relPath)
	if err != nil {
		return errCase(name, err)
	}

	success := strings.Contains(got, "[X] task one") && !strings.Contains(got, "2026-01-01")
	cr := test.CaseResult{
		Name:     name,
		Expected: "- [X] task one",
		Actual:   got,
		Success:  success,
	}
	if !success {
		cr.Error = "date stamp was not removed"
	}
	return cr
}

// caseConvertToMarkdown mirrors handleAPIConvertFileToMarkdown: convert dokuwiki content via
// dokuwikiconverter and save under a .md path.
func caseConvertToMarkdown() test.CaseResult {
	name := "convert-to-markdown"
	relPath := testPath("legacy.dw")

	dokuwikiContent := "====== Heading ======\n\n**bold text**\n"
	if err := writeFile(relPath, dokuwikiContent); err != nil {
		return errCase(name, err)
	}

	markdown := dokuwikiconverter.NewWithFilePath(relPath).ConvertToMarkdown(dokuwikiContent)
	mdPath := strings.TrimSuffix(relPath, filepath.Ext(relPath)) + ".md"
	if err := writeFile(mdPath, markdown); err != nil {
		return errCase(name, err)
	}

	got, err := readFile(mdPath)
	if err != nil {
		return errCase(name, err)
	}

	success := strings.Contains(got, "# Heading") && strings.Contains(got, "**bold text**")
	cr := test.CaseResult{
		Name:     name,
		Expected: "# Heading ... **bold text**",
		Actual:   got,
		Success:  success,
	}
	if !success {
		cr.Error = "dokuwiki content not converted to expected markdown"
	}
	return cr
}

// caseEditorTOC posts unsaved markdown to the live TOC api of the codemirror editor and gets
// the headings the rendered page would show.
func caseEditorTOC() test.CaseResult {
	name := "editor-toc"
	ts := httptest.NewServer(server.NewRouter())
	defer ts.Close()
	form := url.Values{"content": {"---\ntitle: x\n---\n# One **bold** [x](a.md)\n```\n# code\n```\n## Two\n"}}
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/editor/toc", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		return errCase(name, err)
	}
	defer resp.Body.Close()
	var got []parser.EditorHeading
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		return errCase(name, err)
	}
	want := []parser.EditorHeading{{Level: 1, Text: "One bold x", Line: 3}, {Level: 2, Text: "Two", Line: 7}}
	success := resp.StatusCode == http.StatusOK && slices.Equal(got, want)
	cr := test.CaseResult{Name: name, Expected: "the headings of the posted markdown as the rendered page shows them", Actual: fmt.Sprintf("%d %+v", resp.StatusCode, got), Success: success}
	if !success {
		cr.Error = fmt.Sprintf("want %+v", want)
	}
	return cr
}
