// Package configmanager - App configuration from environment variables
package configmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"knov/internal/logging"
	"knov/internal/utils"

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
	MetadataRebuildInterval string
	HideMarkdown            bool
	HideText                bool
	HideList                bool
	HideTodo                bool
	HideFilter              bool
	HideIndex               bool
	HideImage               bool
	HideVideo               bool
	HidePDF                 bool
	HideOfficeDocuments     bool
	HideArchives            bool
	HideExecutables         bool
	HideScripts             bool
	ShowHiddenFiles         bool
	HomeDashboard           string
	UseExtensionTodo        bool
	UseExtensionList        bool
	UseExtensionIndex       bool
	KanbanPrefix            string
	KanbanStatuses          []string
	KanbanColumns           []string
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
		MetadataStorageProvider: getEnv("KNOV_METADATA_STORAGE_PROVIDER", "sqlite"),
		CacheStorageProvider:    getEnv("KNOV_CACHE_STORAGE_PROVIDER", "sqlite"),
		SearchStorageProvider:   getEnv("KNOV_SEARCH_STORAGE_PROVIDER", "sqlite"),
		SearchEngine:            getEnv("KNOV_SEARCH_ENGINE", "repository"),
		LinkRegex: []string{
			"\\[\\[([^\\]]+)\\]\\]",
			"\\[([^\\]]+)\\]\\([^)]+\\)",
			"\\[\\[([^|]+)\\|[^\\]]+\\]\\]",
			"\\{\\{([^}]+)\\}\\}",
		},
		CronjobInterval:         getEnv("KNOV_CRONJOB_INTERVAL", "5m"),
		SearchIndexInterval:     getEnv("KNOV_SEARCH_INDEX_INTERVAL", "15m"),
		MetadataRebuildInterval: getEnv("KNOV_METADATA_REBUILD_INTERVAL", "60m"),
		HideMarkdown:            getBoolEnv("KNOV_HIDE_MARKDOWN", false),
		HideText:                getBoolEnv("KNOV_HIDE_TEXT", false),
		HideList:                getBoolEnv("KNOV_HIDE_LIST", false),
		HideTodo:                getBoolEnv("KNOV_HIDE_TODO", false),
		HideFilter:              getBoolEnv("KNOV_HIDE_FILTER", false),
		HideIndex:               getBoolEnv("KNOV_HIDE_INDEX", false),
		HideImage:               getBoolEnv("KNOV_HIDE_IMAGE", false),
		HideVideo:               getBoolEnv("KNOV_HIDE_VIDEO", false),
		HidePDF:                 getBoolEnv("KNOV_HIDE_PDF", false),
		HideOfficeDocuments:     getBoolEnv("KNOV_HIDE_OFFICE_DOCUMENTS", false),
		HideArchives:            getBoolEnv("KNOV_HIDE_ARCHIVES", false),
		HideExecutables:         getBoolEnv("KNOV_HIDE_EXECUTABLES", false),
		HideScripts:             getBoolEnv("KNOV_HIDE_SCRIPTS", false),
		ShowHiddenFiles:         getBoolEnv("KNOV_SHOW_HIDDEN_FILES", false),
		HomeDashboard:           getEnv("KNOV_HOME_DASHBOARD", ""),
		UseExtensionTodo:        getBoolEnv("KNOV_USE_EXTENSION_TODO", false),
		UseExtensionList:        getBoolEnv("KNOV_USE_EXTENSION_LIST", false),
		UseExtensionIndex:       getBoolEnv("KNOV_USE_EXTENSION_INDEX", false),
		KanbanPrefix:            getEnv("KNOV_KANBAN_PREFIX", "kb"),
		KanbanStatuses:          getStringListEnv("KNOV_KANBAN_STATUS", []string{"inbox", "inprogress", "blocked", "archive"}),
		KanbanColumns:           getStringListEnv("KNOV_KANBAN_COLUMNS", []string{"inbox", "inprogress", "blocked"}),
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

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return strings.ToLower(value) == "true"
	}
	return defaultValue
}

func getStringListEnv(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				result = append(result, t)
			}
		}
		return result
	}
	return defaultValue
}

// GetKanbanPrefix returns the kanban tag prefix
func GetKanbanPrefix() string {
	return appConfig.KanbanPrefix
}

// GetKanbanStatuses returns all possible kanban statuses
func GetKanbanStatuses() []string {
	return appConfig.KanbanStatuses
}

// GetKanbanColumns returns the visible kanban columns
func GetKanbanColumns() []string {
	return appConfig.KanbanColumns
}

// KanbanStatusTag returns the full tag for a given status
func KanbanStatusTag(status string) string {
	return appConfig.KanbanPrefix + "-status-" + status
}

// IsKanbanTag returns true if a tag is a kanban status tag
func IsKanbanTag(tag string) bool {
	return strings.HasPrefix(tag, appConfig.KanbanPrefix+"-status-")
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

// IsFileTypeHidden checks if a specific editor type should be hidden
func IsFileTypeHidden(editorType string) bool {
	switch strings.ToLower(editorType) {
	case "markdown-editor":
		return appConfig.HideMarkdown
	case "textarea-editor":
		return appConfig.HideText
	case "list-editor":
		return appConfig.HideList
	case "todo-editor":
		return appConfig.HideTodo
	case "filter-editor":
		return appConfig.HideFilter
	case "index-editor":
		return appConfig.HideIndex
	default:
		return false
	}
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

// UpdateEnvFile updates the .env file and immediately applies the change to the
// in-memory appConfig so settings take effect without a restart.
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

	if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return err
	}

	// apply to live config immediately — no restart needed
	applyEnvToAppConfig(key, value)
	return nil
}

// applyEnvToAppConfig updates the in-memory appConfig for any writable env key.
// Mirrors InitAppConfig so every UpdateEnvFile call is reflected instantly.
func applyEnvToAppConfig(key, value string) {
	b := strings.ToLower(value) == "true"
	switch key {
	case "KNOV_DATA_PATH":
		appConfig.DataPath = value
	case "KNOV_GIT_REPO_URL":
		appConfig.GitRepoURL = value
	case "KNOV_LOG_LEVEL":
		appConfig.LogLevel = value
		SetLogLevel(value)
	case "KNOV_HIDE_MARKDOWN":
		appConfig.HideMarkdown = b
	case "KNOV_HIDE_TEXT":
		appConfig.HideText = b
	case "KNOV_HIDE_LIST":
		appConfig.HideList = b
	case "KNOV_HIDE_TODO":
		appConfig.HideTodo = b
	case "KNOV_HIDE_FILTER":
		appConfig.HideFilter = b
	case "KNOV_HIDE_INDEX":
		appConfig.HideIndex = b
	case "KNOV_HIDE_IMAGE":
		appConfig.HideImage = b
	case "KNOV_HIDE_VIDEO":
		appConfig.HideVideo = b
	case "KNOV_HIDE_PDF":
		appConfig.HidePDF = b
	case "KNOV_HIDE_OFFICE_DOCUMENTS":
		appConfig.HideOfficeDocuments = b
	case "KNOV_HIDE_ARCHIVES":
		appConfig.HideArchives = b
	case "KNOV_HIDE_EXECUTABLES":
		appConfig.HideExecutables = b
	case "KNOV_HIDE_SCRIPTS":
		appConfig.HideScripts = b
	case "KNOV_SHOW_HIDDEN_FILES":
		appConfig.ShowHiddenFiles = b
	case "KNOV_USE_EXTENSION_TODO":
		appConfig.UseExtensionTodo = b
	case "KNOV_USE_EXTENSION_LIST":
		appConfig.UseExtensionList = b
	case "KNOV_USE_EXTENSION_INDEX":
		appConfig.UseExtensionIndex = b
	}
	// fields like ServerPort, StoragePath, providers are intentionally excluded —
	// they require a restart to take effect safely.
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

func ExtensionForEditor(editorType string) string {
	switch editorType {
	case "todo":
		return utils.Ternary(appConfig.UseExtensionTodo, ".todo", ".md")
	case "list":
		return utils.Ternary(appConfig.UseExtensionList, ".list", ".md")
	case "index":
		return utils.Ternary(appConfig.UseExtensionIndex, ".index", ".md")
	default:
		return ".md"
	}
}
