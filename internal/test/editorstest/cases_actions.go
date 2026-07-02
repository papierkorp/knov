package editorstest

import (
	"path/filepath"
	"strings"

	"knov/internal/contentHandler"
	"knov/internal/dokuwikiconverter"
	"knov/internal/files"
	"knov/internal/parser"
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
	if err := saveMetadata(relPath, files.EditorTypeToastUI); err != nil {
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
	if err := saveMetadata(relPath, files.EditorTypeToastUI); err != nil {
		return errCase(name, err)
	}

	content, err := readFile(relPath)
	if err != nil {
		return errCase(name, err)
	}
	updated, err := parser.CycleTodoStateAtLine([]byte(content), 0)
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
