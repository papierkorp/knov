package backup

import (
	"time"

	"github.com/robfig/cron/v3"
)

// ParseCronSchedule parses a standard 5-field cron expression ("minute hour day-of-month month
// day-of-week", e.g. "0 18 * * *" for daily at 18:00) for use with AutoBackupDue.
func ParseCronSchedule(expr string) (cron.Schedule, error) {
	return cron.ParseStandard(expr)
}

// AutoBackupDue reports whether profile's next scheduled backup is due on target: true when
// profile has never created a set on target yet, or when schedule's next occurrence after the
// newest set it did create has already passed. Only sets carrying profile's own tag (see
// RunProfile/HasProfileTag) count - a manual backup, or one created by a different profile, must
// never push back when this profile's next backup is due.
//
// Driving this off the newest such set's own timestamp (rather than tracking "did today's slot
// already fire") is what lets a device that's only powered on part of each day (e.g. a USB stick)
// still catch up reliably: if the device was off across one or more scheduled occurrences, the
// next check after it's back on finds schedule.Next(lastBackup) already in the past and backs up
// immediately, instead of waiting for a slot that only ever lands outside its usage window.
func AutoBackupDue(target BackupTarget, schedule cron.Schedule, profile string) (bool, error) {
	names, err := target.List()
	if err != nil {
		return false, err
	}

	var newest time.Time
	found := false
	for _, n := range names {
		if !HasProfileTag(n, profile) {
			continue
		}
		t, err := ParseSetTime(n)
		if err != nil {
			continue
		}
		if !found || t.After(newest) {
			newest = t
			found = true
		}
	}

	if !found {
		return true, nil
	}
	return !schedule.Next(newest).After(time.Now()), nil
}
