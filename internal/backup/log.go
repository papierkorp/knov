package backup

import (
	"sort"
	"time"

	"knov/internal/logging"
)

// LogEntry describes one row of the backup/restore history for display: what happened and when,
// plus whether the set it refers to is still available on target (a set can be rotated away, or
// a restore can reference one deleted since, but the fact something happened is kept regardless
// - see Event) and, if available, its default/locked state.
type LogEntry struct {
	Kind      EventKind   `json:"kind"`
	Source    EventSource `json:"source"` // "" for entries logged before this field existed
	Set       string      `json:"set"`
	Time      time.Time   `json:"time"`
	Available bool        `json:"available"`
	Default   bool        `json:"default"`
	Locked    bool        `json:"locked"`
}

// Log returns the full backup/restore history for target, newest first, enriched with each
// referenced set's current availability/default/locked state.
func Log(target BackupTarget) ([]LogEntry, error) {
	events, err := target.Events()
	if err != nil {
		return nil, err
	}
	names, err := target.List()
	if err != nil {
		return nil, err
	}
	available := make(map[string]bool, len(names))
	for _, n := range names {
		available[n] = true
	}

	loggedBackup := make(map[string]bool, len(events))
	entries := make([]LogEntry, 0, len(events)+len(names))
	for _, e := range events {
		entry := LogEntry{Kind: e.Kind, Source: e.Source, Set: e.Set, Time: e.Time, Available: available[e.Set]}
		if e.Kind == EventBackup {
			loggedBackup[e.Set] = true
		}
		entry.Default, entry.Locked = logEntryFlags(target, e.Set, entry.Available)
		entries = append(entries, entry)
	}

	// sets that exist but predate event logging (or survived a lost log file) still get a row,
	// timestamped from the set's own name, rather than silently disappearing from the log
	for _, n := range names {
		if loggedBackup[n] {
			continue
		}
		t, err := ParseSetTime(n)
		if err != nil {
			continue
		}
		isDefault, locked := logEntryFlags(target, n, true)
		entries = append(entries, LogEntry{Kind: EventBackup, Set: n, Time: t, Available: true, Default: isDefault, Locked: locked})
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Time.After(entries[j].Time) })
	return entries, nil
}

func logEntryFlags(target BackupTarget, name string, available bool) (isDefault, locked bool) {
	if !available {
		return false, false
	}
	locked, err := target.Locked(name)
	if err != nil {
		logging.LogWarning(logging.KeyApp, "backup: failed to check lock on %s: %v", name, err)
	}
	return IsDefaultSet(name), locked
}
