// Package logging provides centralized logging functions
package logging

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// shouldLog checks if message should be logged based on environment variable
func shouldLog(messageLevel string) bool {
	configLevel := os.Getenv("KNOV_LOG_LEVEL")
	if configLevel == "" {
		configLevel = "info"
	}
	switch configLevel {
	case "debug":
		return true // show all logs
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

func getCaller() string {
	pc, file, _, ok := runtime.Caller(2)
	if !ok {
		return "unknown - unknown"
	}

	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown - unknown"
	}

	// Extract function name
	funcName := fn.Name()
	if idx := strings.LastIndex(funcName, "."); idx >= 0 {
		funcName = funcName[idx+1:]
	}

	// Extract filename
	fileName := filepath.Base(file)

	return fileName + " - " + funcName
}

// LogDebug logs debug messages
func LogDebug(format string, args ...any) {
	if shouldLog("debug") {
		log.Printf("debug [%s]: "+format, append([]any{getCaller()}, args...)...)
	}
}

// LogInfo logs info messages
func LogInfo(format string, args ...any) {
	if shouldLog("info") {
		log.Printf("info [%s]: "+format, append([]any{getCaller()}, args...)...)
	}
}

// LogWarning logs warning messages
func LogWarning(format string, args ...any) {
	if shouldLog("warning") {
		log.Printf("warning [%s]: "+format, append([]any{getCaller()}, args...)...)
	}
}

// LogError logs error messages
func LogError(format string, args ...any) {
	if shouldLog("error") {
		log.Printf("error [%s]: "+format, append([]any{getCaller()}, args...)...)
	}
}

var (
	loggers    = make(map[string]*log.Logger)
	loggersMux sync.RWMutex
)

// LogBuilder returns a logger for a specific key that writes to logs/key.log
func LogBuilder(key string) *log.Logger {
	loggersMux.RLock()
	if logger, exists := loggers[key]; exists {
		loggersMux.RUnlock()
		return logger
	}
	loggersMux.RUnlock()

	loggersMux.Lock()
	defer loggersMux.Unlock()

	// double check after acquiring write lock
	if logger, exists := loggers[key]; exists {
		return logger
	}

	// use same base directory logic as configmanager
	baseDir := "."
	exePath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(exePath)
		// check if running from go build cache (go run)
		if !strings.Contains(execDir, "go-build") {
			baseDir = execDir
		}
	}

	logsDir := filepath.Join(baseDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		log.Printf("failed to create logs directory %s: %v", logsDir, err)
		return log.New(os.Stdout, "", log.LstdFlags)
	}

	// create log file for this key
	logFile := filepath.Join(logsDir, key+".log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("failed to create log file %s: %v", logFile, err)
		return log.New(os.Stdout, "", log.LstdFlags)
	}

	// create logger that writes to both file and stdout
	multiWriter := io.MultiWriter(file, os.Stdout)
	logger := log.New(multiWriter, "["+key+"] ", log.LstdFlags)

	// log the file location for reference
	log.Printf("created debug logger '%s' writing to: %s", key, logFile)

	loggers[key] = logger
	return logger
}
