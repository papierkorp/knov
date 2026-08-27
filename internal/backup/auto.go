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

// AutoBackupDue reports whether a new scheduled full backup is due on target: true when no full
// set exists yet, or when schedule's next occurrence after the newest existing full set has
// already passed. Only full sets count - a manually-triggered partial backup (e.g. "just
// metadata") must not push back when the next scheduled full backup is due.
//
// Driving this off the newest set's own timestamp (rather than tracking "did today's slot already
// fire") is what lets a device that's only powered on part of each day (e.g. a USB stick) still
// catch up reliably: if the device was off across one or more scheduled occurrences, the next
// check after it's back on finds schedule.Next(lastBackup) already in the past and backs up
// immediately, instead of waiting for a slot that only ever lands outside its usage window.
func AutoBackupDue(target BackupTarget, schedule cron.Schedule) (bool, error) {
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
	return !schedule.Next(newestFull).After(time.Now()), nil
}
