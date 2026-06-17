package logging

import (
	"sync"
	"time"
)

const ringBufferSize = 500

// LogEntry is a single structured log record held in the ring buffer.
type LogEntry struct {
	Time    time.Time
	Level   string
	Caller  string
	Message string
}

var (
	ring    [ringBufferSize]LogEntry
	ringPos int
	ringLen int
	ringMu  sync.Mutex
)

func addToRing(entry LogEntry) {
	ringMu.Lock()
	ring[ringPos] = entry
	ringPos = (ringPos + 1) % ringBufferSize
	if ringLen < ringBufferSize {
		ringLen++
	}
	ringMu.Unlock()
}

// GetRecentEntries returns up to n most recent log entries, oldest first.
func GetRecentEntries(n int) []LogEntry {
	ringMu.Lock()
	defer ringMu.Unlock()

	count := ringLen
	if n < count {
		count = n
	}

	entries := make([]LogEntry, count)
	start := (ringPos - ringLen + ringBufferSize) % ringBufferSize
	offset := ringLen - count
	for i := 0; i < count; i++ {
		entries[i] = ring[(start+offset+i)%ringBufferSize]
	}
	return entries
}

// HasFileLogging reports whether file logging is configured.
func HasFileLogging() bool {
	fileWriterMux.Lock()
	defer fileWriterMux.Unlock()
	return fileWriter != nil
}

// GetLogFilePath returns the path of the current log file, or empty string if not configured.
func GetLogFilePath() string {
	fileWriterMux.Lock()
	fw := fileWriter
	fileWriterMux.Unlock()
	if fw == nil {
		return ""
	}
	return fw.path
}
