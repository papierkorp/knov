// Package configmanager - App configuration from environment variables
package configmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"knov/internal/logging"

	"github.com/go-git/go-git/v5"
)

// -----------------------------------------------------------------------------
// ------------------------------- App Config ---------------------------------
// -----------------------------------------------------------------------------

var appConfig AppConfig

// AppConfig contains environment-based application configuration
type AppConfig struct {
	DataPath            string
	ThemesPath          string
	ConfigPath          string
	ServerPort          string
	LogLevel            string
	GitRepoURL          string
	SearchEngine        string
	LinkRegex           []string
	CronjobInterval     string
	SearchIndexInterval string

	// storage configuration
	StorageConfigProvider   string
	StorageMetadataProvider string
	StorageCacheProvider    string
	StorageConfigPath       string
	StorageMetadataPath     string
	StorageCachePath        string

	// storage migration (one-time use)
	MigrateStorage             bool
	MigrateConfigOldProvider   string
	MigrateConfigOldPath       string
	MigrateMetadataOldProvider string
	MigrateMetadataOldPath     string
	MigrateCacheOldProvider    string
	MigrateCacheOldPath        string
}

// InitAppConfig initializes app config from environment variables
func InitAppConfig() {
	loadEnvFile()

	baseDir := "."
	exePath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(exePath)
		// check if running from go build cache (go run)
		if !strings.Contains(execDir, "go-build") {
			baseDir = execDir
		}
	}

	appConfig = AppConfig{
		DataPath:     getEnv("KNOV_DATA_PATH", filepath.Join(baseDir, "data")),
		ThemesPath:   getEnv("KNOV_THEMES_PATH", filepath.Join(baseDir, "themes")),
		ConfigPath:   getEnv("KNOV_CONFIG_PATH", filepath.Join(baseDir, "config")),
		ServerPort:   getEnv("KNOV_SERVER_PORT", "1324"),
		LogLevel:     getEnv("KNOV_LOG_LEVEL", "info"),
		GitRepoURL:   getEnv("KNOV_GIT_REPO_URL", ""),
		SearchEngine: getEnv("KNOV_SEARCH_ENGINE", "memory"),
		LinkRegex: []string{
			"\\[\\[([^\\]]+)\\]\\]",
			"\\[([^\\]]+)\\]\\([^)]+\\)",
			"\\[\\[([^|]+)\\|[^\\]]+\\]\\]",
			"\\{\\{([^}]+)\\}\\}",
		},
		CronjobInterval:     getEnv("KNOV_CRONJOB_INTERVAL", "5m"),
		SearchIndexInterval: getEnv("KNOV_SEARCH_INDEX_INTERVAL", "15m"),

		// storage configuration
		StorageConfigProvider:   getEnv("KNOV_STORAGE_CONFIG_PROVIDER", "json"),
		StorageMetadataProvider: getEnv("KNOV_STORAGE_METADATA_PROVIDER", "json"),
		StorageCacheProvider:    getEnv("KNOV_STORAGE_CACHE_PROVIDER", "json"),
		StorageConfigPath:       getEnv("KNOV_STORAGE_CONFIG_PATH", filepath.Join(baseDir, "storage", "config")),
		StorageMetadataPath:     getEnv("KNOV_STORAGE_METADATA_PATH", filepath.Join(baseDir, "storage", "metadata")),
		StorageCachePath:        getEnv("KNOV_STORAGE_CACHE_PATH", filepath.Join(baseDir, "storage", "cache")),

		// storage migration (one-time use)
		MigrateStorage:             getEnv("KNOV_STORAGE_MIGRATE", "") == "true",
		MigrateConfigOldProvider:   getEnv("KNOV_STORAGE_CONFIG_OLD_PROVIDER", ""),
		MigrateConfigOldPath:       getEnv("KNOV_STORAGE_CONFIG_OLD_PATH", ""),
		MigrateMetadataOldProvider: getEnv("KNOV_STORAGE_METADATA_OLD_PROVIDER", ""),
		MigrateMetadataOldPath:     getEnv("KNOV_STORAGE_METADATA_OLD_PATH", ""),
		MigrateCacheOldProvider:    getEnv("KNOV_STORAGE_CACHE_OLD_PROVIDER", ""),
		MigrateCacheOldPath:        getEnv("KNOV_STORAGE_CACHE_OLD_PATH", ""),
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

// GetStorageConfigProvider returns the config storage provider
func GetStorageConfigProvider() string {
	return appConfig.StorageConfigProvider
}

// GetStorageMetadataProvider returns the metadata storage provider
func GetStorageMetadataProvider() string {
	return appConfig.StorageMetadataProvider
}

// GetStorageCacheProvider returns the cache storage provider
func GetStorageCacheProvider() string {
	return appConfig.StorageCacheProvider
}

// GetStorageConfigPath returns the config storage path
func GetStorageConfigPath() string {
	return appConfig.StorageConfigPath
}

// GetStorageMetadataPath returns the metadata storage path
func GetStorageMetadataPath() string {
	return appConfig.StorageMetadataPath
}

// GetStorageCachePath returns the cache storage path
func GetStorageCachePath() string {
	return appConfig.StorageCachePath
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

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
		_, err := git.PlainClone(dataPath, false, &git.CloneOptions{
			URL: appConfig.GitRepoURL,
		})
		if err != nil {
			logging.LogError("failed to clone repository: %v", err)
			return err
		}
		logging.LogInfo("git repository cloned from %s to %s", appConfig.GitRepoURL, dataPath)
	} else {
		_, err := git.PlainInit(dataPath, false)
		if err != nil {
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

// UpdateEnvFile updates .env file with new values
func UpdateEnvFile(key, value string) error {
	envPath := ".env"

	content := ""
	if data, err := os.ReadFile(envPath); err == nil {
		content = string(data)
	}

	lines := strings.Split(content, "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, key+"=") {
			lines[i] = fmt.Sprintf("%s=%s", key, value)
			found = true
			break
		}
	}

	if !found {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	newContent := strings.Join(lines, "\n")
	return os.WriteFile(envPath, []byte(newContent), 0644)
}

func loadEnvFile() {
	envPath := ".env"
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		logging.LogInfo("no .env file found, using environment variables and defaults")
		return
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		logging.LogWarning("failed to read .env file: %v", err)
		return
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			os.Setenv(key, value)
		}
	}

	logging.LogInfo(".env file loaded")
}

func GetThemesPath() string {
	return appConfig.ThemesPath
}

func GetConfigPath() string {
	return appConfig.ConfigPath
}
