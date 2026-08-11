package backup

import "time"

// AutoBackupDue reports whether a new scheduled full backup is due on target: true when no full
// set exists yet, or when the newest one is older than interval. Only full sets count - a
// manually-triggered partial backup (e.g. "just metadata") must not push back when the next
// scheduled full backup is due.
func AutoBackupDue(target BackupTarget, interval time.Duration) (bool, error) {
	names, err := target.List()
	if err != nil {
		return false, err
	}

	var newestFull time.Time
	found := false
	for _, n := range names {
		if !IsFullSet(n) {
			continue
		}
		t, err := ParseSetTime(n)
		if err != nil {
			continue
		}
		if !found || t.After(newestFull) {
			newestFull = t
			found = true
		}
	}

	if !found {
		return true, nil
	}
	return time.Since(newestFull) >= interval, nil
}
