package job

import (
	"cmp"
	"slices"
	"sync"
	"time"
)

const historySize = 50

var (
	historyMu sync.Mutex
	history   [historySize]JobRun
	historyN  int
)

// GetRecentRuns returns up to historySize job runs, newest first.
func GetRecentRuns() []JobRun {
	historyMu.Lock()
	defer historyMu.Unlock()
	total := historyN
	if total > historySize {
		total = historySize
	}
	out := make([]JobRun, total)
	for i := 0; i < total; i++ {
		slot := (historyN - 1 - i + historySize) % historySize
		out[i] = history[slot]
	}
	return out
}

// RunDuration returns a finished run's elapsed time, or 0 if it is still running.
func RunDuration(r JobRun) time.Duration {
	if r.FinishedAt != nil {
		return r.FinishedAt.Sub(r.StartedAt)
	}
	return 0
}

// SortRuns orders runs in place by key ("job", "started", "finished", "duration",
// "status"); dir "desc" reverses. An unknown key leaves the default newest-first order.
func SortRuns(runs []JobRun, key, dir string) {
	end := func(r JobRun) time.Time {
		if r.FinishedAt != nil {
			return *r.FinishedAt
		}
		return time.Time{}
	}
	byKey := map[string]func(a, b JobRun) int{
		"job":      func(a, b JobRun) int { return cmp.Compare(a.Name, b.Name) },
		"started":  func(a, b JobRun) int { return a.StartedAt.Compare(b.StartedAt) },
		"finished": func(a, b JobRun) int { return end(a).Compare(end(b)) },
		"duration": func(a, b JobRun) int { return cmp.Compare(RunDuration(a), RunDuration(b)) },
		"status":   func(a, b JobRun) int { return cmp.Compare(a.Status, b.Status) },
	}
	cmpFn := byKey[key]
	if cmpFn == nil {
		return
	}
	slices.SortStableFunc(runs, func(a, b JobRun) int {
		if dir == "desc" {
			return cmpFn(b, a)
		}
		return cmpFn(a, b)
	})
}

// IsRunning returns true if the named job is currently executing.
func IsRunning(name string) bool {
	historyMu.Lock()
	defer historyMu.Unlock()
	for i := 0; i < historySize; i++ {
		if history[i].Name == name && history[i].Status == JobStatusRunning {
			return true
		}
	}
	return false
}

func recordStart(name string) int {
	historyMu.Lock()
	defer historyMu.Unlock()
	slot := historyN % historySize
	history[slot] = JobRun{Name: name, StartedAt: time.Now(), Status: JobStatusRunning}
	historyN++
	return slot
}

func recordFinish(slot int, status JobStatus, errMsg string, output any) {
	historyMu.Lock()
	defer historyMu.Unlock()
	now := time.Now()
	history[slot].FinishedAt = &now
	history[slot].Status = status
	history[slot].Error = errMsg
	history[slot].Output = output
}
