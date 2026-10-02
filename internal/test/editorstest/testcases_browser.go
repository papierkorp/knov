// Package editorstest - browser-driven cases covering JS/DOM behavior calling Go functions
// directly can't reach: the table editor's undocumented-Tabulator history internals
// (internal/server/render/render_editor_table.go's registerCustomHistoryTypes/
// withGroupedHistory), and the CodeMirror toolbar's click -> window.mdCommands wiring, both
// of which live in vendored JS bundles with no exported Go equivalent. The toolbar case is a
// builtin-theme integration test, not a theme-agnostic one - it hardcodes builtin's toolbar
// markup (see docs/testing.md's "Browser-driven tests" section).
package editorstest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/chromedp/chromedp"

	"knov/internal/files"
	"knov/internal/server"
	"knov/internal/server/render"
	"knov/internal/test"
	"knov/internal/testkit"
)

// caseTablePasteUndoRedo renders the real table editor page (render.RenderTableEditorForm)
// against the real vendored Tabulator build in a headless browser, pastes into existing
// cells, then checks that a single table.undo() fully reverts the whole paste (proving
// registerCustomHistoryTypes' custom 'grouped' undoer/redoer actually collapses the batch
// into one history step) and table.redo() restores it. Skips if no local Chrome/Chromium
// is available.
func caseTablePasteUndoRedo() test.CaseResult {
	name := "table-paste-undo-redo"
	if !testkit.Available() {
		return test.SkipCase(name, "no local chrome/chromium binary found")
	}

	relPath := testPath("table_undo.md")
	initial := "# Undo table\n\n| A | B |\n| --- | --- |\n| 1 | 2 |\n| 3 | 4 |\n"
	if err := writeFile(relPath, initial); err != nil {
		return errCase(name, err)
	}
	if err := saveMetadata(relPath, files.EditorTypeCodeMirror); err != nil {
		return errCase(name, err)
	}

	assets := httptest.NewServer(server.NewRouter())
	defer assets.Close()

	fragment := render.RenderTableEditorForm(relPath, 0)
	page := fmt.Sprintf(`<!DOCTYPE html><html><head>
<link rel="stylesheet" href="%[1]s/static/tabulator-6.5.0.min.css">
<script src="%[1]s/static/tabulator-6.5.0.min.js"></script>
</head><body>%[2]s</body></html>`, assets.URL, fragment)

	harness := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page))
	}))
	defer harness.Close()

	ctx, cancel, err := testkit.NewBrowser(context.Background())
	if err != nil {
		return test.SkipCase(name, err.Error())
	}
	defer cancel()

	var before, afterPaste, afterUndo, afterRedo string
	err = chromedp.Run(ctx,
		chromedp.Navigate(harness.URL),
		chromedp.WaitVisible("#tabulator-container .tabulator-row"),
		chromedp.Evaluate(`JSON.stringify(table.getData())`, &before),
		// paste into the top-left 2x2 without an active range selection - pasteIntoTable
		// clamps to (0,0) when there's no selection, overwriting the two existing rows'
		// cells in place (no column/row growth), which keeps the undo assertion simple
		chromedp.Evaluate(`pasteIntoTable([["9","8"],["7","6"]]); JSON.stringify(table.getData())`, &afterPaste),
		chromedp.Evaluate(`table.undo(); JSON.stringify(table.getData())`, &afterUndo),
		chromedp.Evaluate(`table.redo(); JSON.stringify(table.getData())`, &afterRedo),
	)
	if err != nil {
		return errCase(name, err)
	}

	pastedOK := strings.Contains(afterPaste, `"col0":"9"`) && strings.Contains(afterPaste, `"col1":"8"`) &&
		strings.Contains(afterPaste, `"col0":"7"`) && strings.Contains(afterPaste, `"col1":"6"`)
	undoneInOneStep := afterUndo == before
	redoneOK := afterRedo == afterPaste

	success := pastedOK && undoneInOneStep && redoneOK
	cr := test.CaseResult{
		Name:     name,
		Expected: "paste writes both cells; one table.undo() fully reverts them (grouped history); table.redo() restores the paste",
		Actual:   fmt.Sprintf("before=%s afterPaste=%s afterUndo=%s afterRedo=%s", before, afterPaste, afterUndo, afterRedo),
		Success:  success,
	}
	if !success {
		cr.Error = "paste/undo/redo through Tabulator's history module did not round-trip as expected"
	}
	return cr
}

// caseCodeMirrorToolbarBold renders the real CodeMirror editor page
// (render.RenderCodeMirrorEditorForm) for an empty file against the real vendored CM6 bundle
// in a headless browser, clicks the toolbar's Bold button, and checks the editor content
// becomes "****" - the exact no-selection insert behavior of the bundle's bold command.
// Starting from an empty document sidesteps needing to simulate a text selection: start and
// end are both position 0 either way. Skips if no local Chrome/Chromium is available.
func caseCodeMirrorToolbarBold() test.CaseResult {
	name := "codemirror-toolbar-bold"
	if !testkit.Available() {
		return test.SkipCase(name, "no local chrome/chromium binary found")
	}

	relPath := testPath("codemirror_toolbar.md")
	if err := writeFile(relPath, ""); err != nil {
		return errCase(name, err)
	}
	if err := saveMetadata(relPath, files.EditorTypeCodeMirror); err != nil {
		return errCase(name, err)
	}

	assets := httptest.NewServer(server.NewRouter())
	defer assets.Close()

	fragment := render.RenderCodeMirrorEditorForm(relPath, "")
	page := fmt.Sprintf(`<!DOCTYPE html><html><head>
<script src="%[1]s/static/codemirror6-bundle.min.js"></script>
<script src="%[1]s/static/wiki-autocomplete.js"></script>
</head><body>%[2]s</body></html>`, assets.URL, fragment)

	harness := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page))
	}))
	defer harness.Close()

	ctx, cancel, err := testkit.NewBrowser(context.Background())
	if err != nil {
		return test.SkipCase(name, err.Error())
	}
	defer cancel()

	var before, after string
	err = chromedp.Run(ctx,
		chromedp.Navigate(harness.URL),
		chromedp.WaitVisible(".cm-content", chromedp.ByQuery),
		chromedp.Text(".cm-content", &before, chromedp.ByQuery),
		chromedp.Click(`#component-codemirror-toolbar button[data-cmd="bold"]`, chromedp.ByQuery),
		chromedp.Text(".cm-content", &after, chromedp.ByQuery),
	)
	if err != nil {
		return errCase(name, err)
	}

	before = strings.TrimSpace(before)
	after = strings.TrimSpace(after)
	success := before == "" && after == "****"

	cr := test.CaseResult{
		Name:     name,
		Expected: `empty doc + one click on the Bold button -> "****" (mdCommands.bold's no-selection insert)`,
		Actual:   fmt.Sprintf("before=%q after=%q", before, after),
		Success:  success,
	}
	if !success {
		cr.Error = "toolbar Bold button click did not dispatch the expected mdCommands.bold edit"
	}
	return cr
}
