// Package logging provides centralized logging functions
package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ── keys ───────────────────────────────────────────────────────────────────────

// Key identifies a log destination. KeyApp is the general app.log; every
// other key gets its own rotating file under the logs directory, and never
// duplicates into app.log.
type Key string

const (
	KeyApp             Key = "" // general/default -> app.log
	KeyFileSync        Key = "file-sync"
	KeySearchReindex   Key = "search-reindex"
	KeyMetadataRebuild Key = "metadata-rebuild"
	KeyFullRebuild     Key = "full-rebuild"
	KeyMediaCleanup    Key = "media-cleanup"
	KeyGitRemote       Key = "git-remote"
	KeyDokuwikiExport  Key = "dokuwiki-export"
	KeyPdfExport       Key = "pdf-export"
	KeyRepairLinks     Key = "repair-broken-links"
	KeyDBMigration     Key = "database-migration"
	KeyMetaMigration   Key = "metadata-migration"
	KeyFilterDebug     Key = "filter-debug"
	KeyManualCronjob   Key = "manual-cronjob"
)

// AvailableKeys lists every valid log destination, e.g. for an admin log-viewer dropdown.
var AvailableKeys = []Key{
	KeyApp, KeyFileSync, KeySearchReindex, KeyMetadataRebuild, KeyFullRebuild,
	KeyMediaCleanup, KeyGitRemote, KeyDokuwikiExport, KeyPdfExport, KeyRepairLinks,
	KeyDBMigration, KeyMetaMigration, KeyFilterDebug, KeyManualCronjob,
}

// String returns the key's display/file name ("app" for the default key).
func (k Key) String() string {
	if k == KeyApp {
		return "app"
	}
	return string(k)
}

// ── level filtering ───────────────────────────────────────────────────────────

func shouldLog(messageLevel string) bool {
	configLevel := os.Getenv("KNOV_LOG_LEVEL")
	if configLevel == "" {
		configLevel = "info"
	}
	switch configLevel {
	case "debug":
		return true
	case "info":
		return messageLevel != "debug"
	case "warning":
		return messageLevel == "warning" || messageLevel == "error"
	case "error":
		return messageLevel == "error"
	default:
		return true
	}
}

func shouldLogToFile(messageLevel string) bool {
	if fileWriter == nil {
		return false
	}
	configLevel := os.Getenv("KNOV_LOG_FILE_LEVEL")
	if configLevel == "" {
		configLevel = "info"
	}
	switch configLevel {
	case "debug":
		return true
	case "info":
		return messageLevel != "debug"
	case "warning":
		return messageLevel == "warning" || messageLevel == "error"
	case "error":
		return messageLevel == "error"
	default:
		return true
	}
}

// ── caller helper ─────────────────────────────────────────────────────────────

func getCaller() string {
	pc, file, _, ok := runtime.Caller(2)
	if !ok {
		return "unknown - unknown"
	}

	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown - unknown"
	}

	funcName := fn.Name()
	if idx := strings.LastIndex(funcName, "."); idx >= 0 {
		funcName = funcName[idx+1:]
	}

	fileName := filepath.Base(file)
	return fileName + " - " + funcName
}

// ── rotating app log ──────────────────────────────────────────────────────────

var (
	fileWriter    *rotatingWriter
	fileWriterMux sync.Mutex
)

// resolveRotationLimits reads the shared size/file-count rotation config used
// by every rotating log file (app.log and each per-key log).
func resolveRotationLimits() (maxMB, maxFiles int) {
	maxMB = 10
	if v := os.Getenv("KNOV_LOG_MAX_SIZE_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxMB = n
		}
	}

	maxFiles = 5
	if v := os.Getenv("KNOV_LOG_MAX_FILES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxFiles = n
		}
	}
	return maxMB, maxFiles
}

// Init sets up the rotating file logger. Call once at startup.
func Init() {
	if os.Getenv("KNOV_LOG_FILE_ENABLED") == "false" {
		return
	}

	maxMB, maxFiles := resolveRotationLimits()

	logsDir := resolveLogsDir()
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		log.Printf("logging: failed to create logs dir: %v", err)
		return
	}

	logPath := filepath.Join(logsDir, "app.log")
	rw, err := newRotatingWriter(logPath, maxMB, maxFiles)
	if err != nil {
		log.Printf("logging: failed to open log file: %v", err)
		return
	}

	fileWriterMux.Lock()
	fileWriter = rw
	fileWriterMux.Unlock()

	fmt.Fprintf(rw, "\n════════════════════════════════════════\n session started %s\n════════════════════════════════════════\n\n", formatLogTime(time.Now()))

	log.Printf("logging: file logging enabled, writing to %s (max %dMB, %d files)", logPath, maxMB, maxFiles)
}

func writeToFile(line string) {
	fileWriterMux.Lock()
	fw := fileWriter
	fileWriterMux.Unlock()
	if fw == nil {
		return
	}
	fmt.Fprintln(fw, line)
}

// ── per-key rotating logs ─────────────────────────────────────────────────────

var (
	keyWritersMu sync.Mutex
	keyWriters   = make(map[Key]*rotatingWriter)
)

// getKeyWriter returns (creating and caching if needed) the rotating writer
// for a non-default key, writing to logs/<key>.log.
func getKeyWriter(key Key) *rotatingWriter {
	keyWritersMu.Lock()
	defer keyWritersMu.Unlock()

	if rw, ok := keyWriters[key]; ok {
		return rw
	}

	logsDir := resolveLogsDir()
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		log.Printf("logging: failed to create logs directory %s: %v", logsDir, err)
		return nil
	}

	logFile := filepath.Join(logsDir, key.String()+".log")
	maxMB, maxFiles := resolveRotationLimits()
	rw, err := newRotatingWriter(logFile, maxMB, maxFiles)
	if err != nil {
		log.Printf("logging: failed to create log file %s: %v", logFile, err)
		return nil
	}

	// write session separator so restarts are immediately visible in the file
	fmt.Fprintf(rw, "\n=== session started %s ===\n", formatLogTime(time.Now()))
	keyWriters[key] = rw
	return rw
}

// writeKeyed appends a pre-formatted line to key's destination (app.log for
// KeyApp, its own rotating file otherwise).
func writeKeyed(key Key, line string) {
	if key == KeyApp {
		writeToFile(line)
		return
	}
	rw := getKeyWriter(key)
	if rw == nil {
		return
	}
	fmt.Fprintln(rw, line)
}

// MarkSessionStart writes a "=== session started ===" separator to key's log
// file, so admins can see each logical run (e.g. one cronjob execution)
// collapsed into its own section in the log viewer.
func MarkSessionStart(key Key) {
	sep := fmt.Sprintf("\n=== session started %s ===\n", formatLogTime(time.Now()))
	if key == KeyApp {
		fileWriterMux.Lock()
		fw := fileWriter
		fileWriterMux.Unlock()
		if fw == nil {
			return
		}
		fmt.Fprint(fw, sep)
		return
	}
	if rw := getKeyWriter(key); rw != nil {
		fmt.Fprint(rw, sep)
	}
}

// ── log functions ─────────────────────────────────────────────────────────────

var (
	timeFormatterMu sync.RWMutex
	timeFormatter   func(time.Time) string
)

// SetTimeFormatter sets the function used to format timestamps in log file lines.
// Call this after config is loaded to apply the user's datetime/timezone settings.
func SetTimeFormatter(fn func(time.Time) string) {
	timeFormatterMu.Lock()
	timeFormatter = fn
	timeFormatterMu.Unlock()
}

func formatLogTime(t time.Time) string {
	timeFormatterMu.RLock()
	fn := timeFormatter
	timeFormatterMu.RUnlock()
	if fn != nil {
		return fn(t)
	}
	return t.Format("2006-01-02 15:04:05")
}

func logLine(key Key, level, caller, format string, args ...any) string {
	msg := fmt.Sprintf(format, args...)
	if key == KeyApp {
		return fmt.Sprintf("%s %s [%s]: %s", formatLogTime(time.Now()), level, caller, msg)
	}
	return fmt.Sprintf("%s %s [%s] [%s]: %s", formatLogTime(time.Now()), level, key, caller, msg)
}

// consolePrintf writes directly to stdout rather than through the standard
// "log" package, so it isn't re-captured by the stdlib interceptor
// (InitInterceptor) and duplicated back into app.log - LogDebug/Info/Warning/
// Error already record their own ring buffer entry and file line directly.
func consolePrintf(key Key, level, caller, msg string) {
	ts := formatLogTime(time.Now())
	if key == KeyApp {
		fmt.Fprintf(os.Stdout, "%s %s [%s]: %s\n", ts, level, caller, msg)
		return
	}
	fmt.Fprintf(os.Stdout, "%s %s [%s] [%s]: %s\n", ts, level, key, caller, msg)
}

// LogDebug logs a debug message under key.
func LogDebug(key Key, format string, args ...any) {
	caller := getCaller()
	msg := fmt.Sprintf(format, args...)
	if shouldLog("debug") {
		consolePrintf(key, "debug", caller, msg)
	}
	if shouldLogToFile("debug") {
		writeKeyed(key, logLine(key, "debug", caller, format, args...))
	}
	addToRing(LogEntry{Time: time.Now(), Level: "debug", Key: key, Caller: caller, Message: msg})
}

// LogInfo logs an info message under key.
func LogInfo(key Key, format string, args ...any) {
	caller := getCaller()
	msg := fmt.Sprintf(format, args...)
	if shouldLog("info") {
		consolePrintf(key, "info", caller, msg)
	}
	if shouldLogToFile("info") {
		writeKeyed(key, logLine(key, "info", caller, format, args...))
	}
	addToRing(LogEntry{Time: time.Now(), Level: "info", Key: key, Caller: caller, Message: msg})
}

// LogWarning logs a warning message under key.
func LogWarning(key Key, format string, args ...any) {
	caller := getCaller()
	msg := fmt.Sprintf(format, args...)
	if shouldLog("warning") {
		consolePrintf(key, "warning", caller, msg)
	}
	if shouldLogToFile("warning") {
		writeKeyed(key, logLine(key, "warning", caller, format, args...))
	}
	addToRing(LogEntry{Time: time.Now(), Level: "warning", Key: key, Caller: caller, Message: msg})
}

// LogError logs an error message under key.
func LogError(key Key, format string, args ...any) {
	caller := getCaller()
	msg := fmt.Sprintf(format, args...)
	if shouldLog("error") {
		consolePrintf(key, "error", caller, msg)
	}
	if shouldLogToFile("error") {
		writeKeyed(key, logLine(key, "error", caller, format, args...))
	}
	addToRing(LogEntry{Time: time.Now(), Level: "error", Key: key, Caller: caller, Message: msg})
}

// ── helpers ───────────────────────────────────────────────────────────────────

func resolveBaseDir() string {
	baseDir := "."
	exePath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(exePath)
		if !strings.Contains(execDir, "go-build") {
			baseDir = execDir
		}
	}
	return baseDir
}

func resolveLogsDir() string {
	if v := os.Getenv("KNOV_LOGS_PATH"); v != "" {
		return v
	}
	return filepath.Join(resolveBaseDir(), "logs")
}

// GetLogsDir returns the resolved logs directory path.
func GetLogsDir() string {
	return resolveLogsDir()
}

// GetAllLogFiles returns basenames of all log files in the logs directory, sorted by modification time (newest first).
func GetAllLogFiles() []string {
	dir := resolveLogsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	type fileEntry struct {
		name    string
		modTime time.Time
	}
	var files []fileEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".log") && !strings.Contains(name, ".log.") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileEntry{name: name, modTime: info.ModTime()})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})

	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.name
	}
	return names
}
