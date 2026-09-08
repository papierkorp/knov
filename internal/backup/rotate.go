package backup

import (
	"sort"
	"time"

	"knov/internal/logging"
)

type setEntry struct {
	name string
	t    time.Time
}

// listParsedSets returns every backup set on target whose name parses via ParseSetTime, newest
// first. Names that don't parse (not a set this package created) are left alone by Rotate.
func listParsedSets(target BackupTarget) ([]setEntry, error) {
	names, err := target.List()
	if err != nil {
		return nil, err
	}

	var entries []setEntry
	for _, n := range names {
		t, err := ParseSetTime(n)
		if err != nil {
			continue
		}
		entries = append(entries, setEntry{n, t})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].t.After(entries[j].t) })
	return entries, nil
}

func deleteSets(target BackupTarget, entries []setEntry, keep map[string]bool) error {
	var deleted int
	for _, e := range entries {
		if keep[e.name] {
			continue
		}
		if err := target.Delete(e.name); err != nil {
			logging.LogWarning(logging.KeyApp, "backup rotate: failed to delete %s: %v", e.name, err)
			continue
		}
		deleted++
	}
	if deleted > 0 {
		logging.LogInfo(logging.KeyApp, "backup rotate: removed %d expired backup set(s)", deleted)
	}
	return nil
}

// Rotate trims target down to backup sets that satisfy at least one of three independent rules:
// locked (kept regardless of age or count - see BackupTarget.Lock), within keepDays of now (any
// kind, default or partial), or among the keepDefault most recent default backups - a long-term
// floor so coming back after months away still leaves something restorable even if daily backups
// lapsed. Partial backups (e.g. "just metadata") get no long-term floor of their own: once a
// partial set falls outside keepDays and isn't locked, it's deleted, so it can never occupy the
// slot a default backup would otherwise have kept. keepDays/keepDefault <= 0 disables that
// individual rule; if both are <= 0, no rule is left to justify keeping anything, so rotation
// itself is disabled entirely (nothing is deleted) rather than that being read as "delete every
// unlocked set".
func Rotate(target BackupTarget, keepDays, keepDefault int) error {
	if keepDays <= 0 && keepDefault <= 0 {
		return nil
	}

	entries, err := listParsedSets(target)
	if err != nil {
		return err
	}
	locked, err := target.LockedNames()
	if err != nil {
		return err
	}

	now := time.Now()
	keep := make(map[string]bool, len(entries))
	defaultKept := 0

	for _, e := range entries { // newest first
		isDefault := IsDefaultSet(e.name)
		withinDays := keepDays > 0 && now.Sub(e.t) <= time.Duration(keepDays)*24*time.Hour
		needsFloor := isDefault && defaultKept < keepDefault

		if locked[e.name] || withinDays || needsFloor {
			keep[e.name] = true
		}
		if isDefault && keep[e.name] {
			defaultKept++
		}
	}

	return deleteSets(target, entries, keep)
}
