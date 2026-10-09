// Package kanbantest - browser-driven case covering the one piece of kanban.js
// (themes/builtin/js/kanban.js) that calling internal/kanban directly can't reach: the
// native HTML5 drag-and-drop wiring itself. This is a builtin-theme integration test, not a
// theme-agnostic one - it hardcodes builtin's DOM/JS (see docs/testing.md's "Browser-driven
// tests" section).
package kanbantest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/chromedp/chromedp"

	"knov/internal/configmanager"
	"knov/internal/kanban"
	"knov/internal/pathutils"
	"knov/internal/server"
	"knov/internal/server/render"
	"knov/internal/test"
	"knov/internal/testkit"
)

// fetchCall mirrors one captured window.fetch call from the page's stubbed fetch below.
type fetchCall struct {
	URL  string `json:"url"`
	Body string `json:"body"`
}

// caseDragCardBetweenColumns renders the real board HTML (render.RenderKanbanBoard) and
// loads the real kanban.js into a headless browser, then simulates dragging the seeded
// alpha card from inbox into inprogress via testkit.Drag - the sequence an in-app suite
// structurally can't exercise since it never runs a dragstart/dragover/drop event. window.fetch
// is stubbed so the case doesn't need a configured kanban board wired up server-side; it only
// checks that the drag reaches the DOM and fires the right requests, not that they 200.
// Skips if no local Chrome/Chromium is available.
func caseDragCardBetweenColumns() test.CaseResult {
	name := "drag-card-between-columns"
	if !testkit.Available() {
		return test.SkipCase(name, "no local chrome/chromium binary found")
	}

	board := configmanager.KanbanBoard{FolderPath: testFolder, Slug: "kanbantest-board", DisplayName: "Kanban Test Board"}
	cols, err := kanban.BuildBoard(testFolder, emptyFilterConfig(), "", "")
	if err != nil {
		return errCase(name, err)
	}
	boardHTML := render.RenderKanbanBoard(cols, board)

	assets := httptest.NewServer(server.NewRouter())
	defer assets.Close()

	page := fmt.Sprintf(`<!DOCTYPE html><html><body>
<script>
window.KANBAN_CONFIG = { board: %q };
window.fetchCalls = [];
window.fetch = function(url, opts) {
	window.fetchCalls.push({ url: String(url), body: (opts && opts.body) ? String(opts.body) : '' });
	return Promise.resolve({ ok: true, json: function() { return Promise.resolve({}); }, text: function() { return Promise.resolve(''); } });
};
</script>
%s
<script src="%s/themes/%s/js/kanban.js"></script>
</body></html>`, board.Slug, boardHTML, assets.URL, configmanager.GetTheme())

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

	sourceSel := fmt.Sprintf(`[data-filepath=%q]`, pathutils.DocsPath(testPath(alphaFile)))
	targetColSel := "#kanban-col-inprogress"

	var movedIntoTarget bool
	var callsJSON string
	err = chromedp.Run(ctx,
		chromedp.Navigate(harness.URL),
		chromedp.WaitVisible(sourceSel),
		testkit.Drag(sourceSel, targetColSel),
		// wait for kanbanDrop's fetch/.then chain to fire all 3 calls (move + both order saves)
		// instead of a fixed sleep, so this doesn't flake under load
		chromedp.Poll("window.fetchCalls.length >= 3", nil, chromedp.WithPollingTimeout(5*time.Second)),
		chromedp.Evaluate(fmt.Sprintf(`!!document.querySelector(%q)`, targetColSel+" "+sourceSel), &movedIntoTarget),
		chromedp.Evaluate(`JSON.stringify(window.fetchCalls)`, &callsJSON),
	)
	if err != nil {
		return errCase(name, err)
	}

	var calls []fetchCall
	if err := json.Unmarshal([]byte(callsJSON), &calls); err != nil {
		return errCase(name, err)
	}

	hasCall := func(urlPart, bodyPart string) bool {
		for _, c := range calls {
			if strings.Contains(c.URL, urlPart) && strings.Contains(c.Body, bodyPart) {
				return true
			}
		}
		return false
	}

	movedCard := hasCall("/api/kanban/card/move", "status=inprogress")
	savedOldOrder := hasCall("/api/kanban/"+board.Slug+"/order", "status=inbox")
	savedNewOrder := hasCall("/api/kanban/"+board.Slug+"/order", "status=inprogress")

	success := movedIntoTarget && movedCard && savedOldOrder && savedNewOrder
	cr := test.CaseResult{
		Name:     name,
		Expected: "dragging alpha into the inprogress column moves it in the DOM and posts card/move + order for both columns",
		Actual:   fmt.Sprintf("movedIntoTarget=%t calls=%s", movedIntoTarget, callsJSON),
		Success:  success,
	}
	if !success {
		cr.Error = "kanban.js drag-and-drop wiring did not behave as expected"
	}
	return cr
}
