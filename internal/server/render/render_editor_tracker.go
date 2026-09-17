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
	if filePath != "" {
		fmt.Fprintf(&h, `<a href="%s" target="_blank" class="btn-secondary">%s</a>`, htmlpkg.EscapeString(pathutils.ToFileURL(filePath)), t("view file"))
	}
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
// *tracker.Counter for a row with a running total, live -/+ buttons, and its own
// saved column choices; pass nil for a blank new-counter row that the next save
// will create, which starts with every column checked (tracker.AllColumns). The
// title is always an editable input and a hidden counter_id[] (empty for a new
// row) rides along so save can match rows by id.
func RenderTrackerCounterRow(trackerID string, c *tracker.Counter) string {
	lang := configmanager.GetLanguage()
	t := func(k string, a ...any) string { return translation.SprintfForRequest(lang, k, a...) }

	cid, title := "", ""
	cols := tracker.AllColumns
	if c != nil {
		cid, title = c.ID, c.Title
		cols = c.EffectiveColumns()
	}

	var h strings.Builder
	fmt.Fprintf(&h, `<div class="tracker-counter-row" x-data='{cols: %s}'>`, columnSetAttr(cols))
	fmt.Fprintf(&h, `<input type="hidden" name="counter_id[]" value="%s"/>`, htmlpkg.EscapeString(cid))
	h.WriteString(`<input type="hidden" name="counter_columns[]" :value="JSON.stringify(cols)"/>`)
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

	h.WriteString(`<div class="tracker-menu-wrap" x-data="dropdownMenu()" @click.outside="close()">`)
	fmt.Fprintf(&h, `<button type="button" class="tracker-menu-btn" x-ref="btn" @click="toggle()" title="%s"><i class="fa fa-ellipsis-vertical"></i></button>`, t("actions"))
	h.WriteString(`<div class="tracker-menu" x-ref="menu" :hidden="!open" @click="close()" @scroll.window.capture="close()">`)
	h.WriteString(columnCheckboxesHTML(t))
	h.WriteString(`<hr/>`)
	if c != nil {
		resetVals, _ := json.Marshal(map[string]string{"trackerid": trackerID, "counterid": c.ID})
		fmt.Fprintf(&h, `<button type="button" class="btn-small btn-danger" hx-post="/api/trackers/reset" hx-vals='%s' hx-target="closest .tracker-counter-row" hx-swap="outerHTML" hx-confirm="%s">%s</button>`,
			htmlpkg.EscapeString(string(resetVals)), t("reset this counter to 0?"), t("reset to 0"))
	}
	fmt.Fprintf(&h, `<button type="button" onclick="this.closest('.tracker-counter-row').remove()" class="btn-small btn-secondary">%s</button>`, t("remove"))
	h.WriteString(`</div></div>`)
	h.WriteString(`</div>`)
	return h.String()
}

// columnSetAttr renders cs as a JSON object literal, HTML-escaped for embedding in
// an Alpine x-data='{cols: ...}' attribute (the browser un-escapes attribute values
// before Alpine reads them, so this round-trips safely). ColumnSet is bool-only
// today, so escaping is currently a no-op - kept anyway so this stays safe if a
// string field is ever added to ColumnSet without this call site being revisited.
func columnSetAttr(cs tracker.ColumnSet) string {
	b, _ := json.Marshal(cs)
	return htmlpkg.EscapeString(string(b))
}

// columnCheckboxesHTML renders the 7 ColumnSet toggles, bound via Alpine to the
// enclosing element's x-data='{cols: ...}' scope (RenderTrackerCounterRow's
// per-row scope).
func columnCheckboxesHTML(t func(string, ...any) string) string {
	// @click.stop keeps a checkbox click from bubbling to the enclosing
	// .tracker-menu's own @click="close()" (meant for the one-shot reset/remove
	// buttons below), which would otherwise close the menu after the first toggle.
	var b strings.Builder
	b.WriteString(`<label class="tracker-col-toggle" @click.stop><input type="checkbox" x-model="cols.day24h"/> ` + t("24h") + `</label>`)
	b.WriteString(`<label class="tracker-col-toggle" @click.stop><input type="checkbox" x-model="cols.day7d"/> ` + t("7d") + `</label>`)
	b.WriteString(`<label class="tracker-col-toggle" @click.stop><input type="checkbox" x-model="cols.day30d"/> ` + t("30d") + `</label>`)
	b.WriteString(`<label class="tracker-col-toggle" @click.stop><input type="checkbox" x-model="cols.allTime"/> ` + t("all-time") + `</label>`)
	b.WriteString(`<label class="tracker-col-toggle" @click.stop><input type="checkbox" x-model="cols.daily"/> ` + t("by day") + `</label>`)
	b.WriteString(`<label class="tracker-col-toggle" @click.stop><input type="checkbox" x-model="cols.weekly"/> ` + t("by week") + `</label>`)
	b.WriteString(`<label class="tracker-col-toggle" @click.stop><input type="checkbox" x-model="cols.monthly"/> ` + t("by month") + `</label>`)
	return b.String()
}

// RenderTrackerFileView re-renders a saved tracker's stats table as of now, so the
// rolling-window columns are current rather than frozen at the last write. Returns
// ok=false when relPath is not a saved tracker's paired file.
func RenderTrackerFileView(relPath string) (string, bool) {
	md, ok := tracker.StatsMarkdown(relPath)
	if !ok {
		return "", false
	}
	rendered, err := parser.NewMarkdownHandler().Render([]byte(md), parser.PathlessRender, false)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to render tracker view for %s: %v", relPath, err)
		return "", false
	}
	return string(rendered), true
}
