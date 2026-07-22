package editorstest

import (
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/pathutils"
	"knov/internal/test"
)

// createEditSaveCase covers the raw-content editors (toastui, textarea, codemirror): write
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

func caseToastUICreateEditSave() test.CaseResult {
	return createEditSaveCase("toastui", testPath("toastui.md"), files.EditorTypeToastUI,
		"# ToastUI initial\n", "# ToastUI edited\n")
}

func caseTextareaCreateEditSave() test.CaseResult {
	return createEditSaveCase("textarea", testPath("textarea.md"), files.EditorTypeTextarea,
		"plain text initial", "plain text edited")
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

// caseListCreateEditSave mirrors handleAPISaveListEditor: convert list items to markdown
// (render.ConvertListItemsToMarkdown just joins "- "+content+"\n" per item, replicated
// directly here to avoid pulling in internal/server/render, which imports internal/job -
// importing it from here would cycle back through job's suite wrappers), write, save
// metadata. Note: the real handler tags list saves with EditorTypeTodo (not EditorTypeList)
// - that's existing app behavior, so this case matches it rather than the "correct" type,
// since the suite verifies what the app actually does.
func caseListCreateEditSave() test.CaseResult {
	name := "list"
	relPath := testPath("list") + configmanager.ExtensionForEditor("list")

	initial := "- first item\n- second item\n"
	if err := writeFile(relPath, initial); err != nil {
		return errCase(name, err)
	}
	if err := saveMetadata(relPath, files.EditorTypeTodo); err != nil {
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

// caseTodoCreateEditSave mirrors handleAPISaveTodoEditor: GFM checkbox markdown (state
// prefixes "[ ] "/"[X] " match render.stateToMarkdown's open/done cases, replicated
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
	if err := files.MetaDataSave(&files.Metadata{
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
	if err := saveMetadata(relPath, files.EditorTypeToastUI); err != nil {
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
