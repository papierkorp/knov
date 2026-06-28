package job

import (
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
