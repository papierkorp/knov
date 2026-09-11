// Package tracker stores a set of named counters, each accumulating a net delta
// per calendar day, and renders their totals over rolling windows as a markdown
// table paired with the tracker's config (see configeditor.Kind, same pattern as
// the filter editor).
package tracker

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"knov/internal/configeditor"
	"knov/internal/files"
	"knov/internal/logging"
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

// Counter is one named tally inside a tracker. Days maps a calendar day
// ("2006-01-02") to the net delta accumulated that day. The id is stable so a
// counter can be renamed without losing its recorded days.
type Counter struct {
	ID    string         `json:"id"`
	Title string         `json:"title"`
	Days  map[string]int `json:"days"`
}

// Config is a tracker's stored configuration.
type Config struct {
	Title    string    `json:"title"`
	Counters []Counter `json:"counters"`
}

// CounterInput is one editor row submitted on save: an existing counter (ID set)
// to keep and retitle, or a new counter (ID empty) to create.
type CounterInput struct {
	ID    string
	Title string
}

// store is the shared persistence + paired-file descriptor for the tracker editor.
var store = configeditor.MustNew("tracker/", files.EditorTypeTracker, "tracker")

// mu serializes the load-modify-save cycle so concurrent ticks (rapid +/- clicks)
// never lose an increment to last-write-wins on the whole config.
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

// DeleteConfig removes a tracker from configStorage and its paired file.
func DeleteConfig(id string) error {
	mu.Lock()
	defer mu.Unlock()
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
		if c, ok := prev[row.ID]; ok && !seen[row.ID] {
			c.Title = name
			next = append(next, c)
			seen[row.ID] = true
			continue
		}
		next = append(next, Counter{ID: newCounterID(), Title: name, Days: map[string]int{}})
	}
	config.Counters = next
	return saveLocked(config, id)
}

// Tick adds delta (+1 or -1) to counterID's bucket for today, then saves and
// regenerates. Returns the updated counter.
func Tick(id, counterID string, delta int) (*Counter, error) {
	if delta != 1 && delta != -1 {
		return nil, fmt.Errorf("%w: delta must be +1 or -1", ErrInvalidInput)
	}

	mu.Lock()
	defer mu.Unlock()

	config, err := GetConfig(id)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	counter := config.GetCounter(counterID)
	if counter == nil {
		return nil, fmt.Errorf("%w: counter %q", ErrNotFound, counterID)
	}
	if counter.Days == nil {
		counter.Days = map[string]int{}
	}
	counter.Days[time.Now().Format(dayKey)] += delta

	if err := saveLocked(config, id); err != nil {
		return nil, err
	}
	return counter, nil
}

// Total returns the counter's net count on or after the day of since; pass the
// zero time for all-time.
func Total(c *Counter, since time.Time) int {
	sinceKey := ""
	if !since.IsZero() {
		sinceKey = since.Format(dayKey)
	}
	sum := 0
	for day, n := range c.Days {
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
	config, err := GetConfig(IDFromPath(relPath))
	if err != nil || config == nil {
		return "", false
	}
	return buildStatsMarkdown(config), true
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
	return store.WritePaired(id, []byte(buildStatsMarkdown(config)))
}

// buildStatsMarkdown renders a summary table (net count per counter over the last
// 1/7/30 days and all-time) plus a by-month breakdown.
func buildStatsMarkdown(config *Config) string {
	now := time.Now()

	var sb strings.Builder
	if config.Title != "" {
		fmt.Fprintf(&sb, "# %s\n\n", mdCell(config.Title))
	}

	sb.WriteString("| counter | 24h | 7d | 30d | all-time |\n|---|--:|--:|--:|--:|\n")
	for i := range config.Counters {
		c := &config.Counters[i]
		fmt.Fprintf(&sb, "| %s | %d | %d | %d | %d |\n", mdCell(c.Title),
			Total(c, now.AddDate(0, 0, -1)),
			Total(c, now.AddDate(0, 0, -7)),
			Total(c, now.AddDate(0, 0, -30)),
			Total(c, time.Time{}))
	}

	if months := monthlyTotals(config); len(months) > 0 {
		sb.WriteString("\n## by month\n\n| month")
		for i := range config.Counters {
			fmt.Fprintf(&sb, " | %s", mdCell(config.Counters[i].Title))
		}
		sb.WriteString(" |\n|---")
		for range config.Counters {
			sb.WriteString("|--:")
		}
		sb.WriteString("|\n")
		for _, m := range months {
			fmt.Fprintf(&sb, "| %s", m.key)
			for i := range config.Counters {
				fmt.Fprintf(&sb, " | %d", m.byCounter[config.Counters[i].ID])
			}
			sb.WriteString(" |\n")
		}
	}

	fmt.Fprintf(&sb, "\n_updated: %s_\n", now.Format("2006-01-02 15:04"))
	return sb.String()
}

// mdCell neutralizes the characters that would break a markdown table cell.
func mdCell(s string) string {
	return strings.NewReplacer("|", "\\|", "\n", " ", "\r", "").Replace(s)
}

type monthRow struct {
	key       string
	byCounter map[string]int // counter id -> net total that month
}

func monthlyTotals(config *Config) []monthRow {
	idx := map[string]map[string]int{}
	for i := range config.Counters {
		c := &config.Counters[i]
		for day, n := range c.Days {
			if len(day) < 7 {
				continue
			}
			m := day[:7] // "2006-01"
			if idx[m] == nil {
				idx[m] = map[string]int{}
			}
			idx[m][c.ID] += n
		}
	}
	keys := make([]string, 0, len(idx))
	for k := range idx {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	rows := make([]monthRow, len(keys))
	for i, k := range keys {
		rows[i] = monthRow{key: k, byCounter: idx[k]}
	}
	return rows
}
