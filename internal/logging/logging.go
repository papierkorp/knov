// Package logging provides centralized logging functions
package logging

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
