// Package render - HTMX HTML rendering for the tracker editor
package render

import (
	"encoding/json"
	"fmt"
	htmlpkg "html"
	"strconv"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/tracker"
	"knov/internal/translation"
)

// RenderTrackerEditor renders the tracker edit form. filePath is the paired file
// path (empty for a new tracker); the tracker id is derived from it.
func RenderTrackerEditor(filePath string) (string, error) {
	lang := configmanager.GetLanguage()
	t := func(k string, a ...any) string { return translation.SprintfForRequest(lang, k, a...) }

	id := ""
	if filePath != "" {
		id = tracker.IDFromPath(filePath)
	}
	var config *tracker.Config
	if id != "" {
		var err error
		if config, err = tracker.GetConfig(id); err != nil {
			return "", err
		}
		if config == nil {
			return "", fmt.Errorf("%w: %q", tracker.ErrNotFound, id)
		}
	}

	var h strings.Builder
	h.WriteString(`<div id="component-tracker-editor">`)
	fmt.Fprintf(&h, `<h4>%s</h4>`, t("tracker configuration"))

	h.WriteString(`<form id="tracker-form" hx-post="/api/trackers/save" hx-target="#editor-status">`)
	if id != "" {
		fmt.Fprintf(&h, `<input type="hidden" name="trackerid" value="%s"/>`, htmlpkg.EscapeString(id))
	} else {
		h.WriteString(`<div class="form-group">`)
		fmt.Fprintf(&h, `<label>%s:</label>`, t("tracker name"))
		h.WriteString(GenerateDatalistInput("trackerid-input", "trackerid", "", t("my-tracker"), "/api/files/folder-suggestions", true))
		h.WriteString(`</div>`)
	}

	title := ""
	if config != nil {
		title = config.Title
	}
	h.WriteString(`<div class="form-group">`)
	fmt.Fprintf(&h, `<label>%s:</label>`, t("title"))
	fmt.Fprintf(&h, `<input type="text" name="title" value="%s" class="form-input" placeholder="%s"/>`,
		htmlpkg.EscapeString(title), t("optional heading"))
	h.WriteString(`</div>`)

	cancelURL := "/"
	if filePath != "" {
		cancelURL = pathutils.ToFileURL(filePath)
	}
	h.WriteString(`<div class="form-actions">`)
	fmt.Fprintf(&h, `<button type="submit" class="btn-primary">%s</button>`, t("save tracker"))
	fmt.Fprintf(&h, `<button type="button" hx-post="/api/trackers/add-counter" hx-target="#tracker-counters" hx-swap="beforeend" class="btn-secondary">%s</button>`, t("add counter"))
	fmt.Fprintf(&h, `<a href="%s" role="button" class="btn-secondary">%s</a>`, htmlpkg.EscapeString(cancelURL), t("cancel"))
	h.WriteString(`</div>`)

	h.WriteString(`<div id="tracker-counters">`)
	if config != nil {
		for i := range config.Counters {
			h.WriteString(RenderTrackerCounterRow(id, &config.Counters[i]))
		}
	}
	h.WriteString(`</div>`)

	if id == "" {
		fmt.Fprintf(&h, `<p class="tracker-hint">%s</p>`, t("name and save the tracker, then use +/- to log changes"))
	}
	h.WriteString(`</form>`)
	h.WriteString(`<div id="editor-status"></div>`)
	h.WriteString(`</div>`)
	return h.String(), nil
}

// RenderTrackerCounterRow renders one counter editor row. Pass a persisted
// *tracker.Counter for a row with a running total and live -/+ buttons; pass nil
// for a blank new-counter row that the next save will create. The title is always
// an editable input and a hidden counter_id[] (empty for a new row) rides along so
// save can match rows by id.
func RenderTrackerCounterRow(trackerID string, c *tracker.Counter) string {
	lang := configmanager.GetLanguage()
	t := func(k string, a ...any) string { return translation.SprintfForRequest(lang, k, a...) }

	cid, title := "", ""
	if c != nil {
		cid, title = c.ID, c.Title
	}

	var h strings.Builder
	h.WriteString(`<div class="tracker-counter-row">`)
	fmt.Fprintf(&h, `<input type="hidden" name="counter_id[]" value="%s"/>`, htmlpkg.EscapeString(cid))
	fmt.Fprintf(&h, `<input type="text" name="counter_title[]" value="%s" class="form-input tracker-counter-input" placeholder="%s" required/>`,
		htmlpkg.EscapeString(title), t("counter name"))

	if c != nil {
		vals := func(d int) string {
			b, _ := json.Marshal(map[string]string{
				"trackerid": trackerID,
				"counterid": c.ID,
				"delta":     strconv.Itoa(d),
			})
			return htmlpkg.EscapeString(string(b))
		}
		h.WriteString(`<span class="tracker-stepper">`)
		fmt.Fprintf(&h, `<button type="button" class="btn-secondary" hx-post="/api/trackers/tick" hx-vals='%s' hx-target="closest .tracker-counter-row" hx-swap="outerHTML">&minus;</button>`, vals(-1))
		fmt.Fprintf(&h, `<span class="tracker-counter-total">%d</span>`, tracker.Total(c, time.Time{}))
		fmt.Fprintf(&h, `<button type="button" class="btn-secondary" hx-post="/api/trackers/tick" hx-vals='%s' hx-target="closest .tracker-counter-row" hx-swap="outerHTML">+</button>`, vals(1))
		h.WriteString(`</span>`)
	}

	fmt.Fprintf(&h, `<button type="button" onclick="this.closest('.tracker-counter-row').remove()" class="tracker-remove-btn" title="%s"><i class="fa fa-times"></i></button>`, t("remove"))
	h.WriteString(`</div>`)
	return h.String()
}

// RenderTrackerFileView re-renders a saved tracker's stats table as of now, so the
// rolling-window columns are current rather than frozen at the last write. Returns
// ok=false when relPath is not a saved tracker's paired file.
func RenderTrackerFileView(relPath string) (string, bool) {
	md, ok := tracker.StatsMarkdown(relPath)
	if !ok {
		return "", false
	}
	rendered, err := parser.NewMarkdownHandler().Render([]byte(md), parser.PathlessRender)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to render tracker view for %s: %v", relPath, err)
		return "", false
	}
	return string(rendered), true
}
