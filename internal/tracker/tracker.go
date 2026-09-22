// Package tracker stores a set of named counters, each accumulating a net delta
// per calendar day (see trackerStorage), and renders their totals over rolling
// windows as a markdown table paired with the tracker's small title/columns
// config (see configeditor.Kind, same pattern as the filter editor).
package tracker

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"knov/internal/configeditor"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/trackerStorage"
)

// dayKey is the bucket key layout: one net delta per counter per calendar day.
const dayKey = "2006-01-02"

// ErrInvalidInput is a client error (e.g. a delta other than +1/-1); ErrNotFound
// means the tracker id or counter id does not exist. Both map to 4xx - any other
// error from this package is a storage failure and is 5xx.
var (
	ErrInvalidInput = errors.New("invalid tracker input")
	ErrNotFound     = errors.New("tracker not found")
)

// ColumnSet controls which figures a counter's generated markdown shows: the
// 24h/7d/30d/all-time totals and the by-day/by-week/by-month breakdowns, each
// independently.
type ColumnSet struct {
	Day24h  bool `json:"day24h"`
	Day7d   bool `json:"day7d"`
	Day30d  bool `json:"day30d"`
	AllTime bool `json:"allTime"`
	Daily   bool `json:"daily"`
	Weekly  bool `json:"weekly"`
	Monthly bool `json:"monthly"`
}

// AllColumns is the ColumnSet with every figure enabled - a counter's effective
// display before it's ever been explicitly configured, and a brand new row's
// starting checkbox state.
var AllColumns = ColumnSet{Day24h: true, Day7d: true, Day30d: true, AllTime: true, Daily: true, Weekly: true, Monthly: true}

// Counter is one named tally inside a tracker. Its day->delta history lives in
// trackerStorage, keyed by (trackerID, ID) - not here - since it's the
// unbounded, ever-growing half of a tracker; only its small, static metadata
// is part of the config blob. The id is stable so a counter can be renamed
// without losing its recorded days.
//
// Columns only takes effect once ColumnsConfigured is true; SetMeta sets both
// together on every save, so a counter saved before ColumnSet existed - or never
// saved through the editor at all - keeps showing everything (AllColumns) rather
// than a zero-value Columns being misread as "hide everything". Once configured,
// Columns is authoritative even when every field is false: that's how a user
// deliberately hides all figures for a counter.
type Counter struct {
	ID                string    `json:"id"`
	Title             string    `json:"title"`
	Columns           ColumnSet `json:"columns"`
	ColumnsConfigured bool      `json:"columnsConfigured"`
}

// EffectiveColumns returns c.Columns if it's been explicitly set (ColumnsConfigured),
// otherwise AllColumns.
func (c *Counter) EffectiveColumns() ColumnSet {
	if !c.ColumnsConfigured {
		return AllColumns
	}
	return c.Columns
}

// Config is a tracker's stored configuration.
type Config struct {
	Title    string    `json:"title"`
	Counters []Counter `json:"counters"`
}

// CounterInput is one editor row submitted on save: an existing counter (ID set)
// to keep and retitle, or a new counter (ID empty) to create.
type CounterInput struct {
	ID      string
	Title   string
	Columns ColumnSet
}

// store is the shared persistence + paired-file descriptor for the tracker editor.
var store = configeditor.MustNew("tracker/", files.EditorTypeTracker, "tracker")

// mu serializes the load-modify-save cycle of a tracker's title/counter-list
// metadata (SetMeta, DeleteConfig) so concurrent editor saves never lose one to
// last-write-wins on the whole config blob. Day-delta ticks don't need it -
// trackerStorage.AddDelta is its own atomic operation.
var mu sync.Mutex

// TrackerIndexPath returns the docs-relative path of the file paired with a tracker.
func TrackerIndexPath(id string) string { return store.PairedPath(id) }

// IDFromPath recovers a tracker id from its paired file's docs-relative path.
func IDFromPath(relPath string) string { return store.IDFromPath(relPath) }

// GetConfig loads a tracker configuration from configStorage, or nil when absent.
func GetConfig(id string) (*Config, error) {
	data, err := store.Get(id)
	if err != nil || data == nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tracker config: %w", err)
	}
	return &c, nil
}

// GetCounter returns a pointer to the counter with the given id, or nil.
func (c *Config) GetCounter(counterID string) *Counter {
	for i := range c.Counters {
		if c.Counters[i].ID == counterID {
			return &c.Counters[i]
		}
	}
	return nil
}

// GetAllTrackers returns all stored tracker ids.
func GetAllTrackers() ([]string, error) { return store.List() }

// saveLocked persists a tracker config and regenerates its paired markdown table.
// The caller must hold mu across its load + saveLocked (hence unexported).
func saveLocked(config *Config, id string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tracker config: %w", err)
	}
	if err := store.Set(id, data); err != nil {
		return fmt.Errorf("failed to save tracker config: %w", err)
	}
	if err := generateIndex(id, config); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to generate tracker index for %s: %v", id, err)
	}
	logging.LogInfo(logging.KeyApp, "saved tracker: %s", id)
	return nil
}

// DeleteConfig removes a tracker from configStorage and its paired file, along
// with every counter's recorded days.
func DeleteConfig(id string) error {
	mu.Lock()
	defer mu.Unlock()
	if err := trackerStorage.DeleteTracker(id); err != nil {
		return fmt.Errorf("failed to delete tracker days: %w", err)
	}
	return store.Delete(id)
}

// SetMeta replaces the tracker title and reconciles its counter list against the
// editor rows: existing counters (matched by id) are kept and retitled, rows with
// no id become new counters, and counters whose row was removed are hard-deleted
// along with their recorded days. A missing tracker is created.
func SetMeta(id, title string, rows []CounterInput) error {
	mu.Lock()
	defer mu.Unlock()

	config, err := GetConfig(id)
	if err != nil {
		return err
	}
	if config == nil {
		config = &Config{}
	}
	config.Title = strings.TrimSpace(title)

	prev := make(map[string]Counter, len(config.Counters))
	for _, c := range config.Counters {
		prev[c.ID] = c
	}

	next := make([]Counter, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		name := strings.TrimSpace(row.Title)
		if name == "" {
			continue
		}
		// The editor always submits every row's checkbox state (see
		// RenderTrackerCounterRow), so any save marks Columns authoritative -
		// including a deliberate all-false selection.
		if c, ok := prev[row.ID]; ok && !seen[row.ID] {
			c.Title = name
			c.Columns = row.Columns
			c.ColumnsConfigured = true
			next = append(next, c)
			seen[row.ID] = true
			continue
		}
		next = append(next, Counter{ID: newCounterID(), Title: name, Columns: row.Columns, ColumnsConfigured: true})
	}
	config.Counters = next
	for prevID := range prev {
		if !seen[prevID] {
			if err := trackerStorage.ResetCounter(id, prevID); err != nil {
				logging.LogWarning(logging.KeyApp, "failed to delete tracker counter days for %s/%s: %v", id, prevID, err)
			}
		}
	}
	return saveLocked(config, id)
}

// mutateCounter loads id's config, applies mutate to counterID's counter, then
// regenerates the paired stats file - the common load/find/regenerate skeleton
// shared by Tick, TickDay and Reset. mutate itself is responsible for persisting
// its change to trackerStorage; unlike the tracker's title/counter-list, day
// deltas don't need mu since trackerStorage's own operations are atomic. Returns
// the updated counter and its new all-time total.
func mutateCounter(id, counterID string, mutate func(*Counter) error) (*Counter, int, error) {
	config, err := GetConfig(id)
	if err != nil {
		return nil, 0, err
	}
	if config == nil {
		return nil, 0, fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	counter := config.GetCounter(counterID)
	if counter == nil {
		return nil, 0, fmt.Errorf("%w: counter %q", ErrNotFound, counterID)
	}
	if err := mutate(counter); err != nil {
		return nil, 0, err
	}

	if err := generateIndex(id, config); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to generate tracker index for %s: %v", id, err)
	}
	total, err := Total(id, counterID, time.Time{})
	if err != nil {
		logging.LogWarning(logging.KeyApp, "failed to compute tracker counter total for %s/%s: %v", id, counterID, err)
	}
	return counter, total, nil
}

// Tick adds delta (+1 or -1) to counterID's bucket for today, then regenerates
// the paired stats file. Returns the updated counter and its new all-time total.
func Tick(id, counterID string, delta int) (*Counter, int, error) {
	if delta != 1 && delta != -1 {
		return nil, 0, fmt.Errorf("%w: delta must be +1 or -1", ErrInvalidInput)
	}
	return mutateCounter(id, counterID, func(c *Counter) error {
		return trackerStorage.AddDelta(id, c.ID, time.Now().Format(dayKey), delta)
	})
}

// TickDay adds delta to counterID's bucket for an arbitrary day instead of today,
// then regenerates the paired stats file. Unlike Tick, delta isn't restricted to
// +1/-1: this is for backdating sample/test data, not the live +/- editor buttons.
func TickDay(id, counterID string, day time.Time, delta int) (*Counter, int, error) {
	return mutateCounter(id, counterID, func(c *Counter) error {
		return trackerStorage.AddDelta(id, c.ID, day.Format(dayKey), delta)
	})
}

// Reset zeroes counterID's recorded days back to 0, then regenerates the paired
// stats file. Returns the updated counter.
func Reset(id, counterID string) (*Counter, int, error) {
	return mutateCounter(id, counterID, func(c *Counter) error {
		return trackerStorage.ResetCounter(id, c.ID)
	})
}

// Total returns counterID's net count in trackerID on or after the day of since;
// pass the zero time for all-time.
func Total(trackerID, counterID string, since time.Time) (int, error) {
	days, err := trackerStorage.GetDays(trackerID, counterID)
	if err != nil {
		return 0, err
	}
	return total(days, since), nil
}

// total sums days' net delta on or after the day of since; pass the zero time
// for all-time.
func total(days map[string]int, since time.Time) int {
	sinceKey := ""
	if !since.IsZero() {
		sinceKey = since.Format(dayKey)
	}
	sum := 0
	for day, n := range days {
		if sinceKey == "" || day >= sinceKey {
			sum += n
		}
	}
	return sum
}

// StatsMarkdown returns the tracker stats table as markdown, current as of now,
// when relPath is a saved tracker's paired file. ok is false otherwise. Used to
// re-render the view live so the rolling-window columns never go stale.
func StatsMarkdown(relPath string) (md string, ok bool) {
	id := IDFromPath(relPath)
	config, err := GetConfig(id)
	if err != nil || config == nil {
		return "", false
	}
	return buildStatsMarkdown(id, config), true
}

// newCounterID returns a short random hex id for a counter.
func newCounterID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// generateIndex writes the stats table as the tracker's paired markdown file.
func generateIndex(id string, config *Config) error {
	return store.WritePaired(id, []byte(buildStatsMarkdown(id, config)))
}

// buildStatsMarkdown renders one section per counter - a heading, a small table
// with only the totals its ColumnSet has enabled, and its own by-day/by-week/
// by-month breakdowns when enabled - since counters can each show a different
// set of figures.
func buildStatsMarkdown(id string, config *Config) string {
	now := time.Now()

	var sb strings.Builder
	if config.Title != "" {
		fmt.Fprintf(&sb, "# %s\n\n", mdCell(config.Title))
	}

	for i := range config.Counters {
		c := &config.Counters[i]
		days, err := trackerStorage.GetDays(id, c.ID)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "failed to load tracker days for %s/%s: %v", id, c.ID, err)
			days = map[string]int{}
		}
		cols := c.EffectiveColumns()
		fmt.Fprintf(&sb, "## %s\n\n", mdCell(c.Title))

		var headers, cells, aligns []string
		addCol := func(header, cell, align string) {
			headers = append(headers, header)
			cells = append(cells, cell)
			aligns = append(aligns, align)
		}
		if cols.Day24h {
			addCol("24h", strconv.Itoa(total(days, now.AddDate(0, 0, -1))), "--:")
		}
		if cols.Day7d {
			addCol("7d", strconv.Itoa(total(days, now.AddDate(0, 0, -7))), "--:")
		}
		if cols.Day30d {
			addCol("30d", strconv.Itoa(total(days, now.AddDate(0, 0, -30))), "--:")
		}
		if cols.AllTime {
			addCol("all-time", strconv.Itoa(total(days, time.Time{})), "--:")
		}

		var months []periodRow
		if cols.Monthly {
			months = counterMonthlyTotals(days)
			if len(months) > 1 {
				totals := make([]int, len(months))
				for i, m := range months {
					totals[i] = m.total
				}
				addCol("trend", sparkline(totals), "---")
			}
		}

		if len(headers) > 0 {
			fmt.Fprintf(&sb, "| %s |\n|%s|\n| %s |\n\n",
				strings.Join(headers, " | "), strings.Join(aligns, "|"), strings.Join(cells, " | "))
		}

		if cols.Daily {
			writeBreakdownTable(&sb, "day", counterDailyTotals(days))
		}
		if cols.Weekly {
			writeBreakdownTable(&sb, "week", counterWeeklyTotals(days))
		}
		if cols.Monthly {
			writeBreakdownTable(&sb, "month", months)
		}
	}

	fmt.Fprintf(&sb, "_updated: %s_\n", now.Format("2006-01-02 15:04"))
	return sb.String()
}

// mdCell neutralizes the characters that would break a markdown table cell.
func mdCell(s string) string {
	return strings.NewReplacer("|", "\\|", "\n", " ", "\r", "").Replace(s)
}

type periodRow struct {
	key   string
	total int
}

// writeBreakdownTable appends a "<label> | total" table for rows, or nothing when
// rows is empty.
func writeBreakdownTable(sb *strings.Builder, label string, rows []periodRow) {
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(sb, "| %s | total |\n|---|--:|\n", label)
	for _, r := range rows {
		fmt.Fprintf(sb, "| %s | %d |\n", r.key, r.total)
	}
	sb.WriteString("\n")
}

// counterPeriodTotals sums days into periodRows keyed by keyFn(day), oldest
// first. A day keyFn maps to "" is skipped rather than grouped under one
// no-key bucket.
func counterPeriodTotals(days map[string]int, keyFn func(day string) string) []periodRow {
	idx := map[string]int{}
	for day, n := range days {
		key := keyFn(day)
		if key == "" {
			continue
		}
		idx[key] += n
	}
	keys := make([]string, 0, len(idx))
	for k := range idx {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	rows := make([]periodRow, len(keys))
	for i, k := range keys {
		rows[i] = periodRow{key: k, total: idx[k]}
	}
	return rows
}

// counterDailyTotals returns days as-is, oldest first - each day is already its
// own net-delta bucket, so no grouping is needed.
func counterDailyTotals(days map[string]int) []periodRow {
	return counterPeriodTotals(days, func(day string) string { return day })
}

// counterWeeklyTotals sums days by ISO year-week ("2006-W02"), oldest first.
func counterWeeklyTotals(days map[string]int) []periodRow {
	return counterPeriodTotals(days, func(day string) string {
		t, err := time.Parse(dayKey, day)
		if err != nil {
			return ""
		}
		year, week := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	})
}

// counterMonthlyTotals sums days by calendar month ("2006-01"), oldest first.
func counterMonthlyTotals(days map[string]int) []periodRow {
	return counterPeriodTotals(days, func(day string) string {
		if len(day) < 7 {
			return ""
		}
		return day[:7]
	})
}

// sparklineBars are the unicode block levels sparkline scales values into, lowest
// to highest.
var sparklineBars = []rune("▁▂▃▄▅▆▇█")

// sparkline renders values (oldest first) as one block character per value, scaled
// between the lowest and highest value. All-equal values (including a single
// value) render at the middle bar, since there's no range to scale against.
func sparkline(values []int) string {
	lo, hi := values[0], values[0]
	for _, v := range values[1:] {
		lo = min(lo, v)
		hi = max(hi, v)
	}

	bars := make([]rune, len(values))
	span := hi - lo
	for i, v := range values {
		if span == 0 {
			bars[i] = sparklineBars[len(sparklineBars)/2]
			continue
		}
		bars[i] = sparklineBars[(v-lo)*(len(sparklineBars)-1)/span]
	}
	return string(bars)
}
