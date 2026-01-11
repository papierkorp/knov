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
	DataPath                string
	ThemesPath              string
	StoragePath             string
	ServerPort              string
	LogLevel                string
	GitRepoURL              string
	ConfigStorageProvider   string
	MetadataStorageProvider string
	CacheStorageProvider    string
	SearchStorageProvider   string
	SearchEngine            string
	LinkRegex               []string
	CronjobInterval         string
	SearchIndexInterval     string
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
		DataPath:                getEnv("KNOV_DATA_PATH", filepath.Join(baseDir, "data")),
		ThemesPath:              getEnv("KNOV_THEMES_PATH", filepath.Join(baseDir, "themes")),
		StoragePath:             getEnv("KNOV_STORAGE_PATH", filepath.Join(baseDir, "storage")),
		ServerPort:              getEnv("KNOV_SERVER_PORT", "1324"),
		LogLevel:                getEnv("KNOV_LOG_LEVEL", "info"),
		GitRepoURL:              getEnv("KNOV_GIT_REPO_URL", ""),
		ConfigStorageProvider:   getEnv("KNOV_CONFIG_STORAGE_PROVIDER", "json"),
		MetadataStorageProvider: getEnv("KNOV_METADATA_STORAGE_PROVIDER", "json"),
		CacheStorageProvider:    getEnv("KNOV_CACHE_STORAGE_PROVIDER", "sqlite"),
		SearchStorageProvider:   getEnv("KNOV_SEARCH_STORAGE_PROVIDER", "sqlite"),
		SearchEngine:            getEnv("KNOV_SEARCH_ENGINE", "repository"),
		LinkRegex: []string{
			"\\[\\[([^\\]]+)\\]\\]",
			"\\[([^\\]]+)\\]\\([^)]+\\)",
			"\\[\\[([^|]+)\\|[^\\]]+\\]\\]",
			"\\{\\{([^}]+)\\}\\}",
		},
		CronjobInterval:     getEnv("KNOV_CRONJOB_INTERVAL", "5m"),
		SearchIndexInterval: getEnv("KNOV_SEARCH_INDEX_INTERVAL", "15m"),
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

// GetDataPath returns the data path
func GetDataPath() string {
	return appConfig.DataPath
}

// GetThemesPath returns the themes path
func GetThemesPath() string {
	return appConfig.ThemesPath
}

// GetConfigPath returns the config path
// GetStoragePath returns storage path
func GetStoragePath() string {
	return appConfig.StoragePath
}

// GetConfigStorageProvider returns config storage provider
func GetConfigStorageProvider() string {
	return appConfig.ConfigStorageProvider
}

// GetMetadataStorageProvider returns metadata storage provider
func GetMetadataStorageProvider() string {
	return appConfig.MetadataStorageProvider
}

// GetCacheStorageProvider returns cache storage provider
func GetCacheStorageProvider() string {
	return appConfig.CacheStorageProvider
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
