// Package logging provides centralized logging functions
package logging

import (
	"fmt"
	"io"
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

// Init sets up the rotating file logger. Call once at startup.
func Init() {
	if os.Getenv("KNOV_LOG_FILE_ENABLED") == "false" {
		return
	}

	maxMB := 10
	if v := os.Getenv("KNOV_LOG_MAX_SIZE_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxMB = n
		}
	}

	maxFiles := 5
	if v := os.Getenv("KNOV_LOG_MAX_FILES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxFiles = n
		}
	}

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

	fmt.Fprintf(rw, "\n════════════════════════════════════════\n session started %s\n════════════════════════════════════════\n\n", time.Now().Format("2006-01-02 15:04:05"))

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

func logLine(level, caller, format string, args ...any) string {
	msg := fmt.Sprintf(format, args...)
	return fmt.Sprintf("%s %s [%s]: %s", formatLogTime(time.Now()), level, caller, msg)
}

// LogDebug logs debug messages
func LogDebug(format string, args ...any) {
	caller := getCaller()
	msg := fmt.Sprintf(format, args...)
	if shouldLog("debug") {
		log.Printf("debug [%s]: %s", caller, msg)
	}
	if shouldLogToFile("debug") {
		writeToFile(logLine("debug", caller, format, args...))
	}
	addToRing(LogEntry{Time: time.Now(), Level: "debug", Caller: caller, Message: msg})
}

// LogInfo logs info messages
func LogInfo(format string, args ...any) {
	caller := getCaller()
	msg := fmt.Sprintf(format, args...)
	if shouldLog("info") {
		log.Printf("info [%s]: %s", caller, msg)
	}
	if shouldLogToFile("info") {
		writeToFile(logLine("info", caller, format, args...))
	}
	addToRing(LogEntry{Time: time.Now(), Level: "info", Caller: caller, Message: msg})
}

// LogWarning logs warning messages
func LogWarning(format string, args ...any) {
	caller := getCaller()
	msg := fmt.Sprintf(format, args...)
	if shouldLog("warning") {
		log.Printf("warning [%s]: %s", caller, msg)
	}
	if shouldLogToFile("warning") {
		writeToFile(logLine("warning", caller, format, args...))
	}
	addToRing(LogEntry{Time: time.Now(), Level: "warning", Caller: caller, Message: msg})
}

// LogError logs error messages
func LogError(format string, args ...any) {
	caller := getCaller()
	msg := fmt.Sprintf(format, args...)
	if shouldLog("error") {
		log.Printf("error [%s]: %s", caller, msg)
	}
	if shouldLogToFile("error") {
		writeToFile(logLine("error", caller, format, args...))
	}
	addToRing(LogEntry{Time: time.Now(), Level: "error", Caller: caller, Message: msg})
}

// ── log builder ───────────────────────────────────────────────────────────────

var (
	loggers    = make(map[string]*log.Logger)
	loggersMux sync.RWMutex
)

// LogBuilder returns a named logger that appends to logs/<key>.log.
// A session separator is written each time the logger is first opened.
func LogBuilder(key string) *log.Logger {
	loggersMux.RLock()
	if logger, exists := loggers[key]; exists {
		loggersMux.RUnlock()
		return logger
	}
	loggersMux.RUnlock()

	loggersMux.Lock()
	defer loggersMux.Unlock()

	if logger, exists := loggers[key]; exists {
		return logger
	}

	logsDir := resolveLogsDir()
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		log.Printf("failed to create logs directory %s: %v", logsDir, err)
		return log.New(os.Stdout, "", log.LstdFlags)
	}

	logFile := filepath.Join(logsDir, key+".log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("failed to create log file %s: %v", logFile, err)
		return log.New(os.Stdout, "", log.LstdFlags)
	}

	// write session separator so restarts are immediately visible in the file
	separator := fmt.Sprintf("\n=== session started %s ===\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprint(file, separator)

	multiWriter := io.MultiWriter(file, os.Stdout)
	logger := log.New(multiWriter, "["+key+"] ", log.LstdFlags)

	log.Printf("created logger '%s' writing to: %s", key, logFile)

	loggers[key] = logger
	return logger
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
