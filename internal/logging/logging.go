// Package logging provides centralized logging functions
package logging

import (
	"log"
	"os"
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

// LogDebug logs debug messages
func LogDebug(format string, args ...any) {
	if shouldLog("debug") {
		log.Printf("DEBUG: "+format, args...)
	}
}

// LogInfo logs info messages
func LogInfo(format string, args ...any) {
	if shouldLog("info") {
		log.Printf("INFO: "+format, args...)
	}
}

// LogWarning logs warning messages
func LogWarning(format string, args ...any) {
	if shouldLog("warning") {
		log.Printf("WARNING: "+format, args...)
	}
}

// LogError logs error messages
func LogError(format string, args ...any) {
	if shouldLog("error") {
		log.Printf("ERROR: "+format, args...)
	}
}
