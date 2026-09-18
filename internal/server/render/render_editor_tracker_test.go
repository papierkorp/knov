package render

import (
	"strings"
	"testing"

	"knov/internal/tracker"
)

// RenderTrackerCounterRow's -/+ stepper only round-trips a real counter through the
// tick API, so it must stay separate from the row's own remove/reset actions - this
// guards the row markup that the kebab-menu-in-the-3-dots change touches.
func TestRenderTrackerCounterRowExistingCounter(t *testing.T) {
	c := &tracker.Counter{ID: "abc123", Title: "pushups"}
	html := RenderTrackerCounterRow("mytracker", c, 3)

	if !strings.Contains(html, `value="pushups"`) {
		t.Errorf("row missing counter title input: %s", html)
	}
	if !strings.Contains(html, `hx-post="/api/trackers/tick"`) {
		t.Errorf("row missing tick buttons: %s", html)
	}
	if !strings.Contains(html, "tracker-menu-wrap") || !strings.Contains(html, "tracker-menu-btn") {
		t.Errorf("row missing 3-dot menu wrapper/button: %s", html)
	}
	if !strings.Contains(html, `hx-post="/api/trackers/reset"`) {
		t.Errorf("row missing reset action: %s", html)
	}
	if !strings.Contains(html, `&#34;counterid&#34;:&#34;abc123&#34;`) {
		t.Errorf("reset hx-vals missing counter id: %s", html)
	}
	if !strings.Contains(html, "btn-danger") {
		t.Errorf("reset button should be styled as destructive (btn-danger): %s", html)
	}
	if !strings.Contains(html, "this.closest('.tracker-counter-row').remove()") {
		t.Errorf("row missing remove button inside menu: %s", html)
	}

	menuStart := strings.Index(html, "tracker-menu-wrap")
	removeIdx := strings.Index(html, "this.closest('.tracker-counter-row').remove()")
	if menuStart == -1 || removeIdx == -1 || removeIdx < menuStart {
		t.Errorf("remove button must be nested inside the 3-dot menu, got: %s", html)
	}
}

// A counter saved before ColumnSet existed has ColumnsConfigured false; the row
// must render it as fully-checked (EffectiveColumns' AllColumns fallback), not as
// all-unchecked, or the editor would misrepresent what the generated markdown
// actually shows.
func TestRenderTrackerCounterRowUnconfiguredRendersAsEverythingChecked(t *testing.T) {
	c := &tracker.Counter{ID: "legacy1", Title: "old counter"}
	html := RenderTrackerCounterRow("mytracker", c, 0)

	if !strings.Contains(html, `cols: {&#34;day24h&#34;:true,&#34;day7d&#34;:true,&#34;day30d&#34;:true,&#34;allTime&#34;:true,&#34;daily&#34;:true,&#34;weekly&#34;:true,&#34;monthly&#34;:true}`) {
		t.Errorf("an unconfigured counter should render as all-checked in the Alpine seed, got: %s", html)
	}
}

// Once a counter is explicitly configured, its Columns selection must ride along
// untouched as the row's Alpine seed so the checkboxes reflect exactly what's
// stored, not a default.
func TestRenderTrackerCounterRowRespectsExplicitColumns(t *testing.T) {
	c := &tracker.Counter{ID: "abc123", Title: "pushups", Columns: tracker.ColumnSet{Day24h: true}, ColumnsConfigured: true}
	html := RenderTrackerCounterRow("mytracker", c, 0)

	if !strings.Contains(html, `cols: {&#34;day24h&#34;:true,&#34;day7d&#34;:false,&#34;day30d&#34;:false,&#34;allTime&#34;:false,&#34;daily&#34;:false,&#34;weekly&#34;:false,&#34;monthly&#34;:false}`) {
		t.Errorf("explicit Columns should ride through unchanged, got: %s", html)
	}
	if !strings.Contains(html, `x-model="cols.day24h"`) || !strings.Contains(html, `x-model="cols.monthly"`) {
		t.Errorf("row missing column checkboxes bound to the Alpine cols scope: %s", html)
	}
	if !strings.Contains(html, `name="counter_columns[]"`) {
		t.Errorf("row missing hidden counter_columns[] input for form submission: %s", html)
	}
}

// A counter explicitly configured with every box unchecked must render as
// all-unchecked, not fall back to all-checked - this is what makes "show nothing"
// representable at all; before ColumnsConfigured existed, this state was
// indistinguishable from "never configured".
func TestRenderTrackerCounterRowRespectsExplicitAllFalse(t *testing.T) {
	c := &tracker.Counter{ID: "abc123", Title: "silent", Columns: tracker.ColumnSet{}, ColumnsConfigured: true}
	html := RenderTrackerCounterRow("mytracker", c, 0)

	if !strings.Contains(html, `cols: {&#34;day24h&#34;:false,&#34;day7d&#34;:false,&#34;day30d&#34;:false,&#34;allTime&#34;:false,&#34;daily&#34;:false,&#34;weekly&#34;:false,&#34;monthly&#34;:false}`) {
		t.Errorf("explicit all-false Columns should render as all-unchecked, got: %s", html)
	}
}

// A blank new-counter row (nil counter) has no id to reset or tick yet, so only the
// remove action belongs in its menu.
func TestRenderTrackerCounterRowNewCounter(t *testing.T) {
	html := RenderTrackerCounterRow("mytracker", nil, 0)

	if strings.Contains(html, `hx-post="/api/trackers/tick"`) {
		t.Errorf("blank row should have no tick buttons: %s", html)
	}
	if strings.Contains(html, `hx-post="/api/trackers/reset"`) {
		t.Errorf("blank row has nothing to reset yet: %s", html)
	}
	if !strings.Contains(html, "tracker-menu-wrap") {
		t.Errorf("blank row still needs the 3-dot menu for remove: %s", html)
	}
	if !strings.Contains(html, "this.closest('.tracker-counter-row').remove()") {
		t.Errorf("blank row missing remove button: %s", html)
	}
}
