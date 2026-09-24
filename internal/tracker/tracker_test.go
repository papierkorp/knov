package tracker

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"knov/internal/cacheStorage"
	"knov/internal/configStorage"
	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/metadataStorage"
	"knov/internal/parser"
	"knov/internal/trackerStorage"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "knov-tracker-test")
	if err != nil {
		panic(err)
	}
	configmanager.SetDataAndStoragePaths(dir, dir)
	if err := contentStorage.Init(); err != nil {
		panic(err)
	}
	parser.Init()
	if err := configStorage.Init("json", dir); err != nil {
		panic(err)
	}
	if err := metadataStorage.Init("json", dir); err != nil {
		panic(err)
	}
	if err := cacheStorage.Init("json", dir); err != nil {
		panic(err)
	}
	if err := trackerStorage.Init(true, "sqlite", dir); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestResetZeroesDaysAndKeepsCounter(t *testing.T) {
	id := "reset-tracker"
	if err := SetMeta(id, "", []CounterInput{{Title: "pushups"}}); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	config, err := GetConfig(id)
	if err != nil || config == nil {
		t.Fatalf("GetConfig: %v", err)
	}
	counterID := config.Counters[0].ID

	if _, _, err := Tick(id, counterID, 1); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if _, _, err := Tick(id, counterID, 1); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if total, err := Total(id, counterID, time.Time{}); err != nil || total != 2 {
		t.Fatalf("sanity: total before reset = %d, err = %v, want 2", total, err)
	}

	counter, err := Reset(id, counterID)
	if err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if counter.ID != counterID || counter.Title != "pushups" {
		t.Errorf("Reset returned counter %+v, want id=%s title=pushups", counter, counterID)
	}
	if days, err := trackerStorage.GetDays(id, counterID); err != nil || len(days) != 0 {
		t.Errorf("Reset left days = %v (err %v), want empty", days, err)
	}

	if got, err := Total(id, counterID, time.Time{}); err != nil || got != 0 {
		t.Errorf("Total after reset = %d, err = %v, want 0", got, err)
	}
}

func TestResetUnknownTracker(t *testing.T) {
	if _, err := Reset("does-not-exist", "whatever"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Reset on unknown tracker: err = %v, want ErrNotFound", err)
	}
}

func TestResetUnknownCounter(t *testing.T) {
	id := "reset-tracker-unknown-counter"
	if err := SetMeta(id, "", []CounterInput{{Title: "sit-ups"}}); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	if _, err := Reset(id, "not-a-real-counter-id"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Reset on unknown counter: err = %v, want ErrNotFound", err)
	}
}

// An unconfigured counter (ColumnsConfigured false, the zero value for a counter
// that predates ColumnSet or was never saved through the editor) must fall back to
// AllColumns regardless of whatever happens to be sitting in Columns.
func TestEffectiveColumnsFallsBackWhenUnconfigured(t *testing.T) {
	c := &Counter{Columns: ColumnSet{Day24h: true}} // ColumnsConfigured left false
	if got := c.EffectiveColumns(); got != AllColumns {
		t.Errorf("unconfigured counter's EffectiveColumns() = %+v, want AllColumns", got)
	}
}

// Once ColumnsConfigured is true, Columns is authoritative even when every field
// is false - that's how a user deliberately hides all figures for a counter. This
// is the fix for the previous zero-value-means-everything design, which had no way
// to represent "show nothing".
func TestEffectiveColumnsHonorsExplicitAllFalse(t *testing.T) {
	c := &Counter{Columns: ColumnSet{}, ColumnsConfigured: true}
	if got := c.EffectiveColumns(); got != (ColumnSet{}) {
		t.Errorf("explicitly-configured all-false counter's EffectiveColumns() = %+v, want all false", got)
	}
}

func TestEffectiveColumnsHonorsExplicitPartialSelection(t *testing.T) {
	only24h := ColumnSet{Day24h: true}
	c := &Counter{Columns: only24h, ColumnsConfigured: true}
	if got := c.EffectiveColumns(); got != only24h {
		t.Errorf("EffectiveColumns() = %+v, want %+v", got, only24h)
	}
}

// SetMeta must mark every submitted row's columns as configured - including one
// that explicitly unchecks everything - so the "show nothing" case actually
// persists end to end rather than only working at the struct level.
func TestSetMetaMarksColumnsConfiguredEvenWhenAllFalse(t *testing.T) {
	id := "all-false-columns-tracker"
	if err := SetMeta(id, "", []CounterInput{
		{Title: "silent-counter", Columns: ColumnSet{}},
	}); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	config, err := GetConfig(id)
	if err != nil || config == nil {
		t.Fatalf("GetConfig: %v", err)
	}
	c := &config.Counters[0]
	if !c.ColumnsConfigured {
		t.Fatal("SetMeta did not mark the counter's columns as configured")
	}
	if got := c.EffectiveColumns(); got != (ColumnSet{}) {
		t.Errorf("EffectiveColumns() after an explicit all-false save = %+v, want all false", got)
	}

	md := buildStatsMarkdown(id, config)
	for _, unwanted := range []string{"24h", "7d", "30d", "all-time", "month"} {
		if strings.Contains(md, unwanted) {
			t.Errorf("markdown should show nothing for an explicitly all-false counter, found %q:\n%s", unwanted, md)
		}
	}
}

// buildStatsMarkdown renders one table per counter, restricted to that counter's
// own enabled columns - so two counters in the same tracker can show different
// figures, unlike the old single shared table.
func TestBuildStatsMarkdownRendersPerCounterColumns(t *testing.T) {
	id := "columns-tracker"
	if err := SetMeta(id, "", []CounterInput{
		{Title: "today-only", Columns: ColumnSet{Day24h: true}},
		{Title: "full-stats", Columns: ColumnSet{Day24h: true, Day7d: true, Day30d: true, AllTime: true, Monthly: true}},
	}); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	config, err := GetConfig(id)
	if err != nil || config == nil {
		t.Fatalf("GetConfig: %v", err)
	}

	todayOnly := config.GetCounter(config.Counters[0].ID)
	if _, err := TickDay(id, todayOnly.ID, time.Now(), 5); err != nil {
		t.Fatalf("TickDay: %v", err)
	}
	fullStats := config.GetCounter(config.Counters[1].ID)
	if _, err := TickDay(id, fullStats.ID, time.Now(), 7); err != nil {
		t.Fatalf("TickDay: %v", err)
	}

	md := buildStatsMarkdown(id, config)

	if !strings.Contains(md, "## today-only") || !strings.Contains(md, "## full-stats") {
		t.Fatalf("expected one heading per counter, got:\n%s", md)
	}

	todayIdx := strings.Index(md, "## today-only")
	fullIdx := strings.Index(md, "## full-stats")
	todaySection := md[todayIdx:fullIdx]
	fullSection := md[fullIdx:]

	if !strings.Contains(todaySection, "24h") {
		t.Errorf("today-only section missing its enabled 24h column:\n%s", todaySection)
	}
	if strings.Contains(todaySection, "7d") || strings.Contains(todaySection, "30d") || strings.Contains(todaySection, "all-time") || strings.Contains(todaySection, "month") {
		t.Errorf("today-only section should have no other columns/monthly breakdown:\n%s", todaySection)
	}
	for _, want := range []string{"24h", "7d", "30d", "all-time", "month"} {
		if !strings.Contains(fullSection, want) {
			t.Errorf("full-stats section missing %q:\n%s", want, fullSection)
		}
	}
}

// A counter saved before ColumnSet existed has ColumnsConfigured false; it must
// still render the full table - EffectiveColumns' AllColumns fallback keeps
// pre-existing trackers' output from silently going blank.
func TestBuildStatsMarkdownUnconfiguredCounterShowsEverything(t *testing.T) {
	id := "legacy-tracker"
	config := &Config{Counters: []Counter{{ID: "c1", Title: "legacy"}}}
	if err := trackerStorage.AddDelta(id, "c1", time.Now().Format(dayKey), 3); err != nil {
		t.Fatalf("AddDelta: %v", err)
	}
	md := buildStatsMarkdown(id, config)
	for _, want := range []string{"24h", "7d", "30d", "all-time"} {
		if !strings.Contains(md, want) {
			t.Errorf("zero-value Columns should default to showing everything, missing %q:\n%s", want, md)
		}
	}
}
