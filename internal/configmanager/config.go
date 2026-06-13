// Package configmanager - App configuration from environment variables
package configmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"knov/internal/logging"
	"knov/internal/utils"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
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
	LogsPath                string
	ServerPort              string
	LogLevel                string
	GitRemote               string
	GitRemoteBranch         string
	GitAutoPush             bool
	GitPushTimeout          string
	GitUser                 string
	GitPassword             string
	GitToken                string
	GitSSHKey               string
	ConfigStorageProvider   string
	MetadataStorageProvider string
	CacheStorageProvider    string
	SearchStorageProvider   string
	KanbanEventsEnabled     bool
	KanbanEventsProvider    string
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
	AutoCreateTags          []string
	AutoCreateCollections   []string
	KanbanTagColors         map[string]string
	KanbanCardStyles        map[string]string // status → "normal"|"italic"|"highlighted"|"deleted"
	KanbanArchiveStatus     string
	NotifyDuration          int
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
		LogsPath:                getEnv("KNOV_LOGS_PATH", filepath.Join(baseDir, "logs")),
		ServerPort:              getEnv("KNOV_SERVER_PORT", "1324"),
		LogLevel:                getEnv("KNOV_LOG_LEVEL", "info"),
		GitRemote:               getEnv("KNOV_GIT_REMOTE", ""),
		GitRemoteBranch:         getEnv("KNOV_GIT_REMOTE_BRANCH", "main"),
		GitAutoPush:             getBoolEnv("KNOV_GIT_AUTO_PUSH", true),
		GitPushTimeout:          getEnv("KNOV_GIT_PUSH_TIMEOUT", "10s"),
		GitUser:                 getEnv("KNOV_GIT_USER", ""),
		GitPassword:             getEnv("KNOV_GIT_PASSWORD", ""),
		GitToken:                getEnv("KNOV_GIT_TOKEN", ""),
		GitSSHKey:               getEnv("KNOV_GIT_SSH_KEY", ""),
		ConfigStorageProvider:   getEnv("KNOV_CONFIG_STORAGE_PROVIDER", "json"),
		MetadataStorageProvider: getEnv("KNOV_METADATA_STORAGE_PROVIDER", "sqlite"),
		CacheStorageProvider:    getEnv("KNOV_CACHE_STORAGE_PROVIDER", "sqlite"),
		SearchStorageProvider:   getEnv("KNOV_SEARCH_STORAGE_PROVIDER", "sqlite"),
		KanbanEventsEnabled:     getBoolEnv("KNOV_KANBAN_EVENTS_ENABLED", true),
		KanbanEventsProvider:    getEnv("KNOV_KANBAN_EVENTS_STORAGE_PROVIDER", "sqlite"),
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
		HomeDashboard:           getEnv("KNOV_HOME_DASHBOARD", "home"),
		UseExtensionTodo:        getBoolEnv("KNOV_USE_EXTENSION_TODO", false),
		UseExtensionList:        getBoolEnv("KNOV_USE_EXTENSION_LIST", false),
		UseExtensionIndex:       getBoolEnv("KNOV_USE_EXTENSION_INDEX", false),
		KanbanPrefix:            getEnv("KNOV_KANBAN_PREFIX", "kb"),
		KanbanStatuses:          getStringListEnv("KNOV_KANBAN_STATUS", []string{"inbox", "inprogress", "blocked", "archive"}),
		KanbanColumns:           getStringListEnv("KNOV_KANBAN_COLUMNS", []string{"inbox", "inprogress", "blocked"}),
		AutoCreateTags:          getStringListEnv("KNOV_AUTOCREATE_TAGS", []string{}),
		AutoCreateCollections:   getStringListEnv("KNOV_AUTOCREATE_COLLECTIONS", []string{}),
		KanbanTagColors:         getStringMapEnv("KNOV_KANBAN_TAG_COLORS"),
		KanbanCardStyles:        getStringMapEnv("KNOV_KANBAN_CARD_STYLES"),
		KanbanArchiveStatus:     getEnv("KNOV_KANBAN_ARCHIVE_STATUS", "archive"),
		NotifyDuration:          getIntEnv("KNOV_NOTIFY_DURATION", 3500),
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

// GetNotifyDuration returns the notification toast display duration in milliseconds
func GetNotifyDuration() int {
	return appConfig.NotifyDuration
}

// GetKanbanTagColors returns the tag-name → CSS-color map
func GetKanbanTagColors() map[string]string {
	return appConfig.KanbanTagColors
}

// GetKanbanCardStyles returns the kanban-status → card-style map ("normal"|"italic"|"highlighted"|"deleted")
func GetKanbanCardStyles() map[string]string {
	return appConfig.KanbanCardStyles
}

// GetKanbanArchiveStatus returns the status used to archive (hide) cards from the board
func GetKanbanArchiveStatus() string {
	return appConfig.KanbanArchiveStatus
}

// getStringMapEnv parses "key1:val1,key2:val2" into a map
func getStringMapEnv(key string) map[string]string {
	result := make(map[string]string)
	if value := os.Getenv(key); value != "" {
		for _, pair := range strings.Split(value, ",") {
			parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.TrimSpace(parts[1])
				if k != "" && v != "" {
					result[k] = v
				}
			}
		}
	}
	return result
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
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

// GetAutoCreateTags returns tags automatically added to every newly created file
func GetAutoCreateTags() []string {
	return appConfig.AutoCreateTags
}

// GetAutoCreateCollections returns the collections that scope auto-tag behaviour (empty = all)
func GetAutoCreateCollections() []string {
	return appConfig.AutoCreateCollections
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

// GetGitRemote returns the configured git remote URL (empty = local only)
func GetGitRemote() string {
	return appConfig.GitRemote
}

// GetGitRemoteBranch returns the git remote branch name
func GetGitRemoteBranch() string {
	return appConfig.GitRemoteBranch
}

// GetGitAutoPush returns whether auto-push is enabled
func GetGitAutoPush() bool {
	return appConfig.GitAutoPush
}

// GetGitPushTimeout returns the push/pull timeout string
func GetGitPushTimeout() string {
	return appConfig.GitPushTimeout
}

// GetGitAuth returns user, password/token for HTTPS auth (token takes priority)
func GetGitAuth() (user, password string) {
	user = appConfig.GitUser
	if appConfig.GitToken != "" {
		password = appConfig.GitToken
	} else {
		password = appConfig.GitPassword
	}
	return
}

// GetGitSSHKey returns the path to the SSH private key file (empty = use agent or default)
func GetGitSSHKey() string {
	return appConfig.GitSSHKey
}

// GetLogsPath returns the logs path
func GetLogsPath() string {
	return appConfig.LogsPath
}

// GetConfigStorageProvider returns config storage provider
func GetConfigStorageProvider() string {
	return appConfig.ConfigStorageProvider
}

// GetMetadataStorageProvider returns metadata storage provider
func GetMetadataStorageProvider() string {
	return appConfig.MetadataStorageProvider
}

// GetKanbanEventsEnabled returns whether kanban event logging is enabled
func GetKanbanEventsEnabled() bool {
	return appConfig.KanbanEventsEnabled
}

// GetKanbanEventsProvider returns the kanban events storage provider
func GetKanbanEventsProvider() string {
	return appConfig.KanbanEventsProvider
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

	if appConfig.GitRemote != "" {
		var auth transport.AuthMethod
		if appConfig.GitSSHKey != "" {
			var err error
			auth, err = ssh.NewPublicKeysFromFile("git", appConfig.GitSSHKey, "")
			if err != nil {
				logging.LogError("failed to load ssh key for clone: %v", err)
				return err
			}
		}
		_, err := git.PlainClone(dataPath, false, &git.CloneOptions{
			URL:  appConfig.GitRemote,
			Auth: auth,
		})
		if err != nil {
			logging.LogError("failed to clone repository: %v", err)
			return err
		}
		logging.LogInfo("git repository cloned from %s to %s", appConfig.GitRemote, dataPath)
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
	case "KNOV_GIT_REMOTE":
		appConfig.GitRemote = value
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
