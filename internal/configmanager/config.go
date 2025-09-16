// Package configmanager - App configuration from environment variables
package configmanager

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"knov/internal/logging"
)

// -----------------------------------------------------------------------------
// ------------------------------- App Config ---------------------------------
// -----------------------------------------------------------------------------

var appConfig AppConfig

// AppConfig contains environment-based application configuration
type AppConfig struct {
	DataPath        string
	ServerPort      string
	LogLevel        string
	GitRepoURL      string
	MetadataStorage string
	SearchEngine    string
	LinkRegex       []string
}

// InitAppConfig initializes app config from environment variables
func InitAppConfig() {
	appConfig = AppConfig{
		DataPath:        getEnv("KNOV_DATA_PATH", "data"),
		ServerPort:      getEnv("KNOV_SERVER_PORT", "1324"),
		LogLevel:        getEnv("KNOV_LOG_LEVEL", "info"),
		GitRepoURL:      getEnv("KNOV_GIT_REPO_URL", ""),
		MetadataStorage: getEnv("KNOV_METADATA_STORAGE", "json"),
		SearchEngine:    getEnv("KNOV_SEARCH_ENGINE", "memory"),
		LinkRegex: []string{
			"\\[\\[([^\\]]+)\\]\\]",
			"\\[([^\\]]+)\\]\\([^)]+\\)",
			"\\[\\[([^|]+)\\|[^\\]]+\\]\\]",
			"\\{\\{([^}]+)\\}\\}",
		},
	}

	initLogLevel()

	if err := InitGitRepository(); err != nil {
		logging.LogError("failed to initialize git repository: %s", err)
	}

	logging.LogInfo("app config initialized")
}

// GetAppConfig returns the current app config
func GetAppConfig() AppConfig {
	return appConfig
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// -----------------------------------------------------------------------------
// --------------------------------- Log Level ---------------------------------
// -----------------------------------------------------------------------------

func initLogLevel() {
	logLevel := appConfig.LogLevel
	logging.LogInfo("loglevel set to: %s", logLevel)
	os.Setenv("KNOV_LOG_LEVEL", logLevel)
}

// SetLogLevel set log level and update environment
func SetLogLevel(level string) {
	validLevels := []string{"debug", "info", "warning", "error"}

	if !slices.Contains(validLevels, level) {
		logging.LogWarning("invalid log level '%s', falling back to 'info'", level)
		level = "info"
	}

	os.Setenv("KNOV_LOG_LEVEL", level)
	logging.LogInfo("log level updated to: %s", level)
}

// GetMetadataStorageMethod returns storage method
func GetMetadataStorageMethod() string {
	return appConfig.MetadataStorage
}

// GetMetadataLinkRegex returns link regex patterns
func GetMetadataLinkRegex() []string {
	return appConfig.LinkRegex
}

// InitGitRepository initializes git repository based on configuration
func InitGitRepository() error {
	dataPath := appConfig.DataPath
	gitDir := filepath.Join(dataPath, ".git")

	if _, err := os.Stat(gitDir); !os.IsNotExist(err) {
		logging.LogInfo("git repository already exists in %s", dataPath)
		return nil
	}

	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return err
	}

	if appConfig.GitRepoURL != "" {
		cmd := exec.Command("git", "clone", appConfig.GitRepoURL, dataPath)
		if err := cmd.Run(); err != nil {
			logging.LogError("failed to clone repository: %v", err)
			return err
		}
		logging.LogInfo("git repository cloned from %s to %s", appConfig.GitRepoURL, dataPath)
	} else {
		cmd := exec.Command("git", "init")
		cmd.Dir = dataPath
		if err := cmd.Run(); err != nil {
			logging.LogError("failed to initialize git repository: %v", err)
			return err
		}
		logging.LogInfo("local git repository initialized in %s", dataPath)
	}

	return nil
}

// GetSearchEngine ..
func GetSearchEngine() string {
	return appConfig.SearchEngine
}
