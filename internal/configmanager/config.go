// Package configmanager - App configuration from environment variables
package configmanager

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"knov/internal/backup"
	"knov/internal/logging"
	"knov/internal/utils"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// -----------------------------------------------------------------------------
// ------------------------------- App Config ---------------------------------
// -----------------------------------------------------------------------------

var appConfig AppConfig

// AppConfig contains environment-based application configuration. Each field's KNOV_*
// variable, default, description and parsing are declared exactly once, in envdefs.go's
// EnvVarDefs - not here. A field with no corresponding EnvVarDefs entry isn't 1:1 env-backed
// (e.g. LinkRegex is a fixed constant, not user-configurable).
type AppConfig struct {
	DataPath                  string
	ThemesPath                string
	StoragePath               string
	LogsPath                  string
	BackupsPath               string
	ServerPort                string
	ServerHost                string
	GitRemote                 string
	GitRemoteBranch           string
	GitAutoPush               bool
	GitPushTimeout            string
	GitUser                   string
	GitPassword               string
	GitToken                  string
	GitSSHKey                 string
	ConfigStorageProvider     string
	MetadataStorageProvider   string
	CacheStorageProvider      string
	SearchStorageProvider     string
	TrackerStorageProvider    string
	KanbanEventsEnabled       bool
	KanbanEventsProvider      string
	KanbanPrefix              string
	SearchEngine              string
	LinkRegex                 []string
	CronjobInterval           string
	SearchIndexInterval       string
	MetadataRebuildInterval   string
	AutoCreateTags            []AutoCreateTag
	TrackerEnabled            bool
	NotifyDuration            int
	NotifyMinLevel            string
	DefaultEditor             string
	BackupAutoProfiles        []BackupProfile
	BackupRotationKeepDays    int
	BackupRotationKeepDefault int
	BackupS3Endpoint          string
	BackupS3Region            string
	BackupS3Bucket            string
	BackupS3Prefix            string
	BackupS3AccessKey         string
	BackupS3SecretKey         string
	BackupS3UseSSL            bool
	LogFileEnabled            bool
	LogMaxSizeMB              int
	LogMaxFiles               int
	MOTD                      string
}

// KanbanBoard maps a folder to a kanban board with a display name and a stable URL slug.
// FolderSync keeps a card's status tag and its physical folder location in sync with each
// other: moving a card moves the file into FolderPath/<status>/, and moving the file on disk
// into an existing FolderPath/<status>/ folder sets the tag.
type KanbanBoard struct {
	FolderPath  string
	DisplayName string
	Slug        string
	FolderSync  bool
}

// AutoCreateTag applies Tag to every new file created under FolderPath (recursive - also
// covers subfolders). FolderPath == "" means apply to every new file regardless of location.
type AutoCreateTag struct {
	FolderPath string
	Tag        string
}

// BackupProfile is one independently scheduled automatic backup: its own cron expression and
// storage selection. Name is slugified from the configured name since it's stamped onto every set
// the profile creates (see backup.RunProfile) and used to scope that profile's own due-tracking
// (backup.AutoBackupDue) - a blank or post-slugify-duplicate name is rejected by
// getBackupProfilesEnv rather than silently generated/renumbered, so a profile's identity never
// shifts across restarts due to entry order.
type BackupProfile struct {
	Name     string
	Cron     string
	Storages []string // empty = the default backup set (job.DefaultStorageNames) at run time
}

// InitAppConfig initializes app config from environment variables
func InitAppConfig() {
	envMsg, envWarn := loadEnvFile()

	// resolved once here rather than duplicated - logging.ResolveBaseDir is the single
	// implementation, since configmanager already depends on logging
	baseDir := logging.ResolveBaseDir()

	// LinkRegex has no KNOV_* key - it's a fixed constant, not user-configurable. Every other
	// field is populated by applyEnvDefs from EnvVarDefs (envdefs.go), the single place each
	// key, its default and its parsing are declared.
	appConfig = AppConfig{
		LinkRegex: []string{
			"\\[\\[([^\\]]+)\\]\\]",
			"\\[([^\\]]+)\\]\\([^)]+\\)",
			"\\[\\[([^|]+)\\|[^\\]]+\\]\\]",
			"\\{\\{([^}]+)\\}\\}",
		},
	}
	applyEnvDefs(&appConfig, baseDir)

	// Set up file logging as soon as its config is known, so every log line from here on
	// (the loadEnvFile result below, git repo init/clone, "app config initialized") lands in
	// logs/app.log too.
	logging.Init(appConfig.LogFileEnabled, appConfig.LogMaxSizeMB, appConfig.LogMaxFiles)
	logging.InitInterceptor()

	if envWarn {
		logging.LogWarning(logging.KeyApp, "%s", envMsg)
	} else {
		logging.LogInfo(logging.KeyApp, "%s", envMsg)
	}

	initLogLevel()
	initNotifyMinLevel()

	if err := InitGitRepository(); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize git repository: %s", err)
	}

	logging.LogInfo(logging.KeyApp, "app config initialized")
}

// GetAppConfig returns the current app config
func GetAppConfig() AppConfig {
	return appConfig
}

// SetDataAndStoragePaths overrides DataPath and StoragePath in memory only (the .env file on
// disk is left untouched). Used by `knov --start-tests` (see test.PrepareIsolatedStorage) to
// point every storage backend at an isolated knov_temp_test scratch copy before anything is
// initialized, so a headless test run never touches the live paths.
func SetDataAndStoragePaths(dataPath, storagePath string) {
	appConfig.DataPath = dataPath
	appConfig.StoragePath = storagePath
}

// GetNotifyDuration returns the notification toast display duration in milliseconds
func GetNotifyDuration() int {
	return appConfig.NotifyDuration
}

// GetNotifyMinLevel returns the minimum notification severity that still shows a
// toast: "info", "warning", "error" or "off". Lower levels are still written to
// the persistent notification log.
func GetNotifyMinLevel() string {
	return appConfig.NotifyMinLevel
}

// initNotifyMinLevel normalizes KNOV_NOTIFY_MIN_LEVEL (trim + lowercase) and warns
// on an unrecognized value, falling back to "info" so a typo never silently
// changes which notifications toast.
func initNotifyMinLevel() {
	level := strings.ToLower(strings.TrimSpace(appConfig.NotifyMinLevel))
	if !slices.Contains([]string{"info", "warning", "error", "off"}, level) {
		logging.LogWarning(logging.KeyApp, "invalid KNOV_NOTIFY_MIN_LEVEL '%s', falling back to 'info'", appConfig.NotifyMinLevel)
		level = "info"
	}
	appConfig.NotifyMinLevel = level
}

// GetBackupAutoProfiles returns the configured automatic backup profiles - each independently
// scheduled and storage-selected. An empty list means automatic backups are disabled entirely.
func GetBackupAutoProfiles() []BackupProfile {
	return appConfig.BackupAutoProfiles
}

// GetBackupRotationKeepDays returns how many days of backup sets (default or partial) are always
// kept, regardless of count. <= 0 disables this rule.
func GetBackupRotationKeepDays() int {
	return appConfig.BackupRotationKeepDays
}

// GetBackupRotationKeepDefault returns the minimum number of default backup sets always kept
// regardless of age, on top of GetBackupRotationKeepDays. <= 0 disables this rule. Locked
// backup sets are kept regardless of either setting.
func GetBackupRotationKeepDefault() int {
	return appConfig.BackupRotationKeepDefault
}

// GetBackupS3Config returns the S3 backup-storage settings. A non-empty Bucket switches backup
// storage from the local filesystem (KNOV_BACKUPS_PATH) to S3 - see job.DefaultBackupTarget.
func GetBackupS3Config() backup.S3Config {
	return backup.S3Config{
		Endpoint:  appConfig.BackupS3Endpoint,
		Region:    appConfig.BackupS3Region,
		Bucket:    appConfig.BackupS3Bucket,
		Prefix:    appConfig.BackupS3Prefix,
		AccessKey: appConfig.BackupS3AccessKey,
		SecretKey: appConfig.BackupS3SecretKey,
		UseSSL:    appConfig.BackupS3UseSSL,
	}
}

// SetBackupAutoProfiles overrides BackupAutoProfiles in memory only (no .env write) - this is an
// AppConfig field by design, not a live Setting, so there's no production setter. Exists for
// backuptest to exercise job.checkAutoBackup's per-profile enabled/disabled/due branches without a
// real restart; callers must restore the original value themselves.
func SetBackupAutoProfiles(profiles []BackupProfile) {
	appConfig.BackupAutoProfiles = profiles
}

// SetBackupsPath overrides BackupsPath in memory only (no .env write) - lets backuptest point
// job.DefaultBackupTarget at a scratch directory instead of the real KNOV_BACKUPS_PATH.
// Callers must restore the original value themselves.
func SetBackupsPath(path string) {
	appConfig.BackupsPath = path
}

// GetKanbanTagColors returns the tag-name → CSS-color map
func GetKanbanTagColors() map[string]string {
	return parseKeyValues(KanbanTagColors.Get())
}

// GetKanbanCardStyles returns the kanban-status → card-style map ("normal"|"italic"|"highlighted"|"deleted")
func GetKanbanCardStyles() map[string]string {
	return parseKeyValues(KanbanCardStyles.Get())
}

// GetKanbanArchiveStatus returns the status used to archive (hide) cards from the board
func GetKanbanArchiveStatus() string {
	return KanbanArchiveStatus.Get()
}

// GetKanbanAncestorAllowedStatus returns the statuses a descendant card must have for its
// ancestor to be listed in the ancestor filter; empty means no restriction (all allowed)
func GetKanbanAncestorAllowedStatus() []string {
	return KanbanAncestorAllowedStatus.Get()
}

// GetKanbanBoards returns the configured folder-based kanban boards
func GetKanbanBoards() []KanbanBoard {
	return parseKanbanBoards(KanbanBoards.Get(), KanbanFolderSync.Get())
}

// GetKanbanBoardBySlug looks up a configured kanban board by its URL slug
func GetKanbanBoardBySlug(slug string) (KanbanBoard, bool) {
	for _, b := range GetKanbanBoards() {
		if b.Slug == slug {
			return b, true
		}
	}
	return KanbanBoard{}, false
}

// GetKanbanBoardByFolder looks up a configured kanban board by its exact folder path
func GetKanbanBoardByFolder(folderPath string) (KanbanBoard, bool) {
	for _, b := range GetKanbanBoards() {
		if b.FolderPath == folderPath {
			return b, true
		}
	}
	return KanbanBoard{}, false
}

// parseKeyValues parses "key:val" entries into a map, skipping malformed ones
func parseKeyValues(entries []string) map[string]string {
	result := make(map[string]string)
	for _, entry := range entries {
		k, v, ok := strings.Cut(entry, ":")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if ok && k != "" && v != "" {
			result[k] = v
		}
	}
	return result
}

// NormalizeKanbanFolder turns a configured board / folder sync folder into the trimmed,
// forward-slash docs-relative form boards are looked up by (pathutils can't be used here, it
// imports configmanager).
func NormalizeKanbanFolder(folder string) string {
	return strings.Trim(strings.ReplaceAll(strings.TrimSpace(folder), `\`, "/"), "/")
}

// parseKanbanBoards parses "folder/path:Display Name" entries into kanban boards, deriving a
// stable URL slug from each folder path (colliding slugs get a numeric suffix, same scheme as
// header-anchor IDs) and flagging the boards whose folder path is listed in folderSync.
func parseKanbanBoards(entries, folderSync []string) []KanbanBoard {
	var boards []KanbanBoard
	usedSlugs := map[string]int{}
	for _, entry := range entries {
		folderPath, displayName, ok := strings.Cut(entry, ":")
		folderPath = NormalizeKanbanFolder(folderPath)
		displayName = strings.TrimSpace(displayName)
		if !ok || folderPath == "" || displayName == "" {
			continue
		}
		boards = append(boards, KanbanBoard{
			FolderPath:  folderPath,
			DisplayName: displayName,
			Slug:        utils.GenerateID(folderPath, usedSlugs),
			FolderSync:  slices.ContainsFunc(folderSync, func(f string) bool { return NormalizeKanbanFolder(f) == folderPath }),
		})
	}
	return boards
}

// getBackupProfilesEnv parses "name:cron:storages;name:cron:storages" into a list of automatic
// backup profiles - storages is itself comma-separated and, along with its leading ":", may be
// omitted entirely for the default backup set at run time (e.g. "daily:0 0 * * *"). An entry with
// a malformed shape, a blank name, an invalid cron expression, or an unregistered storage name is
// dropped with a warning rather than failing startup or silently renaming/renumbering. Two entries
// colliding on the same name once slugified disable automatic backups entirely (rather than
// silently keeping just the first) - each profile's own due-tracking is scoped by that name (see
// backup.HasProfileTag), so a silent partial drop would leave a schedule the user configured
// quietly not running with no indication why.
func getBackupProfilesEnv(key string) []BackupProfile {
	var profiles []BackupProfile
	seenNames := map[string]bool{}
	value := os.Getenv(key)
	if value == "" {
		return profiles
	}
	for _, entry := range strings.Split(value, ";") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, ":", 3)
		if len(parts) < 2 {
			logging.LogWarning(logging.KeyApp, "%s entry %q is not name:cron[:storages], ignoring", key, entry)
			continue
		}
		rawName := strings.TrimSpace(parts[0])
		if rawName == "" {
			logging.LogWarning(logging.KeyApp, "%s entry %q has no name, ignoring", key, entry)
			continue
		}
		cronExpr := strings.TrimSpace(parts[1])
		if _, err := backup.ParseCronSchedule(cronExpr); err != nil {
			logging.LogWarning(logging.KeyApp, "%s entry %q has invalid cron expression %q, ignoring: %v", key, entry, cronExpr, err)
			continue
		}
		var storagesRaw string
		if len(parts) == 3 {
			storagesRaw = parts[2]
		}
		storages := splitList(storagesRaw)
		registered := backup.RegisteredNames()
		if i := slices.IndexFunc(storages, func(s string) bool { return !slices.Contains(registered, s) }); i != -1 {
			logging.LogWarning(logging.KeyApp, "%s entry %q has unknown storage %q, ignoring", key, entry, storages[i])
			continue
		}
		name := utils.GenerateID(rawName, map[string]int{})
		if seenNames[name] {
			logging.LogWarning(logging.KeyApp, "%s entry %q has duplicate profile name %q, disabling all automatic backup profiles", key, entry, name)
			return nil
		}
		seenNames[name] = true
		profiles = append(profiles, BackupProfile{Name: name, Cron: cronExpr, Storages: storages})
	}
	return profiles
}

func formatBackupProfiles(profiles []BackupProfile) string {
	parts := make([]string, 0, len(profiles))
	for _, p := range profiles {
		parts = append(parts, p.Name+":"+p.Cron+":"+strings.Join(p.Storages, ","))
	}
	return strings.Join(parts, "; ")
}

// getAutoCreateTagsEnv parses "folder/path:tagname, tagname2, other/folder:tagname3" into a
// list of auto-create tag rules. An entry with no ":" is a bare tag applied to every new file;
// an entry with ":" scopes the tag to that folder (and its subfolders).
func getAutoCreateTagsEnv(key string) []AutoCreateTag {
	var result []AutoCreateTag
	value := os.Getenv(key)
	if value == "" {
		return result
	}
	for _, entry := range strings.Split(value, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) == 1 {
			if tag := strings.TrimSpace(parts[0]); tag != "" {
				result = append(result, AutoCreateTag{Tag: tag})
			}
			continue
		}
		folderPath := strings.Trim(strings.TrimSpace(parts[0]), "/")
		tag := strings.TrimSpace(parts[1])
		if tag == "" {
			continue
		}
		result = append(result, AutoCreateTag{FolderPath: folderPath, Tag: tag})
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
		return splitList(value)
	}
	return defaultValue
}

// splitList parses a comma-separated env var value into a trimmed, non-empty-entry list.
// "" yields nil, matching the zero value of an unset []string field.
func splitList(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}

// GetMOTD returns the banner message shown at the top of every page, empty if unset
func GetMOTD() string {
	return appConfig.MOTD
}

// GetKanbanPrefix returns the kanban tag prefix
func GetKanbanPrefix() string {
	return appConfig.KanbanPrefix
}

// GetKanbanStatuses returns all possible kanban statuses
func GetKanbanStatuses() []string {
	return KanbanStatuses.Get()
}

// GetKanbanColumns returns the visible kanban columns
func GetKanbanColumns() []string {
	return KanbanColumns.Get()
}

// GetAutoCreateTags returns the folder-scoped tags applied to newly created files
func GetAutoCreateTags() []AutoCreateTag {
	return appConfig.AutoCreateTags
}

// KanbanStatusTag returns the full tag for a given status
func KanbanStatusTag(status string) string {
	return GetKanbanPrefix() + "-status-" + status
}

// IsKanbanTag returns true if a tag is a kanban status tag
func IsKanbanTag(tag string) bool {
	return strings.HasPrefix(tag, GetKanbanPrefix()+"-status-")
}

func initLogLevel() {
	const key = "KNOV_LOG_LEVEL"
	logLevel := getEnv(key, envVarDefault(key))
	logging.LogInfo(logging.KeyApp, "loglevel set to: %s", logLevel)
	os.Setenv(key, logLevel)
}

// SetLogLevel set log level and update environment
func SetLogLevel(level string) {
	validLevels := []string{"debug", "info", "warning", "error"}

	if !slices.Contains(validLevels, level) {
		logging.LogWarning(logging.KeyApp, "invalid log level '%s', falling back to 'info'", level)
		level = "info"
	}

	os.Setenv("KNOV_LOG_LEVEL", level)
	logging.LogInfo(logging.KeyApp, "log level updated to: %s", level)
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

// GetBackupsPath returns the path backup sets are written to and restored from
func GetBackupsPath() string {
	return appConfig.BackupsPath
}

// GetGitRemote returns the configured git remote URL (empty = local only)
func GetGitRemote() string {
	return appConfig.GitRemote
}

// SetGitRemoteForTest sets the in-memory git remote without touching the .env file.
// Test-only: lets git remote tests point EnsureRemote at a throwaway repo and restore
// the original afterward, without a restart.
func SetGitRemoteForTest(value string) {
	appConfig.GitRemote = value
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

// GetTrackerEnabled returns whether the tracker editor is enabled
func GetTrackerEnabled() bool {
	return appConfig.TrackerEnabled
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
	case "codemirror-editor":
		return HideMarkdown.Get()
	case "list-editor":
		return HideList.Get()
	case "todo-editor":
		return HideTodo.Get()
	case "filter-editor":
		return HideFilter.Get()
	case "tracker-editor":
		return !appConfig.TrackerEnabled || HideTracker.Get()
	case "index-editor":
		return HideIndex.Get()
	case "book-editor":
		return HideBook.Get()
	default:
		return false
	}
}

// Hide scopes: feature areas that can be targeted by a HidePaths or HideFilesByTag entry's
// "::tag1|tag2" suffix (see NewHideMatcher). Callers with no per-scope override
// (media) pass "" instead, which never matches a tag - an entry with tags can
// never hide a path from such a caller, only an entry with no suffix can.
const (
	HideScopeTree     = "tree"
	HideScopeBrowse   = "browse"
	HideScopeOverview = "overview"
	HideScopeSearch   = "search"
	HideScopeFilter   = "filter"
	HideScopeKanban   = "kanban"
	// HideScopeDetail covers the single-file detail data served by GetFilesWithSameTags /
	// GetFilesInSameFolder via the /api/files/overview, /files/same-tags and /files/same-folder
	// routes (see api_files.go, api_links.go). Those routes and their render.RenderSidebarField*
	// helpers are theme-agnostic API surface - it's up to each theme's own templates/JS whether
	// that data ends up in a slide-out panel (as the builtin theme does), a modal, or elsewhere.
	// Despite the route name, this is unrelated to HideScopeOverview below, which gates the
	// unrelated files-listing "overview" view (see FilterByVisibility).
	HideScopeDetail = "detail"
	// HideScopeDashboard is the dashboard tag, collection and folder widgets.
	HideScopeDashboard = "dashboard"
)

// hideScopes lists every recognized hide scope tag.
var hideScopes = []string{HideScopeTree, HideScopeBrowse, HideScopeOverview, HideScopeSearch, HideScopeFilter, HideScopeKanban, HideScopeDetail, HideScopeDashboard}

// ValidateHideScopes rejects a HidePaths/HideFilesByTag entry whose "::tag1|tag2" suffix
// contains an unrecognized scope name, so a typo (e.g. "::serach") fails on save instead of
// silently leaving the file hidden everywhere.
func ValidateHideScopes(entries []string) error {
	for _, entry := range entries {
		_, tags := splitHideEntry(entry)
		for _, tag := range tags {
			if !slices.Contains(hideScopes, tag) {
				return fmt.Errorf("unknown scope %q in %q (known scopes: %s)", tag, entry, strings.Join(hideScopes, ", "))
			}
		}
	}
	return nil
}

// ValidateHidePaths is ValidateHideScopes plus rejecting a pattern segment that isn't a valid
// regexp, so a typo fails on save instead of silently never matching.
func ValidateHidePaths(entries []string) error {
	if err := ValidateHideScopes(entries); err != nil {
		return err
	}
	for _, entry := range entries {
		pattern, _ := splitHideEntry(entry)
		if _, err := compileHidePath(pattern); err != nil {
			return fmt.Errorf("invalid pattern %q: %w", entry, err)
		}
	}
	return nil
}

var (
	kanbanNamePattern     = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	kanbanTagColorPattern = regexp.MustCompile(`^(#[0-9a-fA-F]{3,8}|[a-zA-Z]+|var\(--[\w-]+\))$`)
	kanbanCardStyles      = []string{"normal", "italic", "highlighted", "deleted"}
)

// CheckKanbanPrefix refuses to start while KNOV_KANBAN_PREFIX isn't only letters, digits and _
// (the prefix ends up in tags and html attributes).
func CheckKanbanPrefix() error {
	if !kanbanNamePattern.MatchString(GetKanbanPrefix()) {
		return fmt.Errorf("KNOV_KANBAN_PREFIX %q must only contain letters, digits and _", GetKanbanPrefix())
	}
	return nil
}

// ValidateKanbanArchiveStatus allows an empty value (archive zone disabled) or a valid status name.
func ValidateKanbanArchiveStatus(status string) error {
	if status == "" {
		return nil
	}
	return validateKanbanNames([]string{status})
}

// validateKanbanNames only allows letters, digits and _ in every status name, each at most once.
func validateKanbanNames(names []string) error {
	for i, n := range names {
		if !kanbanNamePattern.MatchString(n) {
			return fmt.Errorf("invalid status %q, only letters, digits and _ are allowed", n)
		}
		if slices.Contains(names[:i], n) {
			return fmt.Errorf("duplicate status %q", n)
		}
	}
	return nil
}

// ValidateKanbanBoards rejects malformed folder/path:Display Name entries, folders outside docs
// and a folder configured twice.
func ValidateKanbanBoards(entries []string) error {
	var folders []string
	for _, entry := range entries {
		folderPath, displayName, ok := strings.Cut(entry, ":")
		if !ok || strings.TrimSpace(displayName) == "" {
			return fmt.Errorf("invalid entry %q (expected folder/path:Display Name)", entry)
		}
		if err := validateKanbanFolder(folderPath); err != nil {
			return err
		}
		folderPath = NormalizeKanbanFolder(folderPath)
		if slices.Contains(folders, folderPath) {
			return fmt.Errorf("duplicate board folder %q", folderPath)
		}
		folders = append(folders, folderPath)
	}
	return nil
}

// ValidateKanbanFolderSync rejects folders outside docs.
func ValidateKanbanFolderSync(folders []string) error {
	for _, f := range folders {
		if err := validateKanbanFolder(f); err != nil {
			return err
		}
	}
	return nil
}

// validateKanbanFolder only accepts a clean path below the docs folder - no .., absolute or
// drive path, and no leading docs/, media/ or files/ that pathutils would strip or reroute.
// Uses forward-slash rules only, so a value is accepted or rejected the same on every OS.
func validateKanbanFolder(folderPath string) error {
	folderPath = NormalizeKanbanFolder(folderPath)
	first, _, _ := strings.Cut(folderPath, "/")
	if strings.Contains(folderPath, ":") || path.Clean(folderPath) != folderPath || slices.Contains([]string{".", "..", "docs", "media", "files"}, first) {
		return fmt.Errorf("invalid folder %q, must be a folder path relative to docs (e.g. projects/work)", folderPath)
	}
	return nil
}

// ValidateKanbanTagColors only allows color names, #hex or var(--name) values, since the color
// ends up in a style attribute.
func ValidateKanbanTagColors(entries []string) error {
	return validateKeyValues(entries, kanbanTagColorPattern.MatchString, "expected tag:color with a color name, #hex or var(--name)")
}

// ValidateKanbanCardStyles only allows the known card styles, since the style ends up in a class.
func ValidateKanbanCardStyles(entries []string) error {
	return validateKeyValues(entries, func(v string) bool { return slices.Contains(kanbanCardStyles, v) }, "expected status:style with style one of "+strings.Join(kanbanCardStyles, ", "))
}

// validateKeyValues rejects "key:value" entries that are malformed or whose value fails valid.
func validateKeyValues(entries []string, valid func(string) bool, hint string) error {
	for _, entry := range entries {
		k, v, ok := strings.Cut(entry, ":")
		if !ok || strings.TrimSpace(k) == "" || !valid(strings.TrimSpace(v)) {
			return fmt.Errorf("invalid entry %q (%s)", entry, hint)
		}
	}
	return nil
}

// HideMatcher holds the HidePaths and HideFilesByTag entries that apply to one scope, compiled
// once - build it once per loop via NewHideMatcher, not per iteration.
type HideMatcher struct {
	paths [][]*regexp.Regexp
	tags  []*regexp.Regexp
}

// NewHideMatcher compiles the hide entries that apply to scope. Each entry is a pattern,
// optionally suffixed with "::tag1|tag2" (e.g. "projects/archive::search|filter"). An entry
// without a suffix applies to every scope, one with a suffix only to the listed scopes - scope ""
// (callers without a per-scope override) therefore only gets unsuffixed entries.
func NewHideMatcher(scope string) *HideMatcher {
	m := &HideMatcher{}
	for _, pattern := range hidePatternsForScope(HidePaths.Get(), scope) {
		if segs, err := compileHidePath(pattern); err == nil {
			m.paths = append(m.paths, segs)
		}
	}
	for _, pattern := range hidePatternsForScope(HideFilesByTag.Get(), scope) {
		if pattern == "" {
			continue
		}
		parts := strings.Split(pattern, "*")
		for i, p := range parts {
			parts[i] = regexp.QuoteMeta(p)
		}
		m.tags = append(m.tags, regexp.MustCompile("(?i)^"+strings.Join(parts, ".*")+"$"))
	}
	return m
}

// PathHidden checks if a relative folder path ("/"-separated, no leading/trailing slash) matches
// a HidePaths pattern. A pattern is "/"-separated; a segment of "*" matches any single path
// segment, any other segment is a case-insensitive regular expression that must fully match one
// segment. A pattern matches if its segments align with any contiguous run of the path's
// segments, e.g. "*/todo" hides every folder named "todo", while "test/todo" only hides the
// "todo" folder inside "test".
func (m *HideMatcher) PathHidden(relDirPath string) bool {
	if relDirPath == "" {
		return false
	}
	pathSegs := strings.Split(relDirPath, "/")
	for _, patternSegs := range m.paths {
		if len(patternSegs) > len(pathSegs) {
			continue
		}
		for start := 0; start+len(patternSegs) <= len(pathSegs); start++ {
			if pathSegmentsMatch(patternSegs, pathSegs[start:start+len(patternSegs)]) {
				return true
			}
		}
	}
	return false
}

// TagHidden reports whether tag matches a HideFilesByTag pattern. Patterns are literal and
// case-insensitive except for "*", which matches any run of characters.
func (m *HideMatcher) TagHidden(tag string) bool {
	for _, re := range m.tags {
		if re.MatchString(tag) {
			return true
		}
	}
	return false
}

// compileHidePath compiles a HidePaths pattern into one regexp per segment (nil for "*").
func compileHidePath(pattern string) ([]*regexp.Regexp, error) {
	segs := strings.Split(strings.Trim(pattern, "/"), "/")
	res := make([]*regexp.Regexp, len(segs))
	for i, seg := range segs {
		if seg == "*" {
			continue
		}
		re, err := regexp.Compile("(?i)^" + seg + "$")
		if err != nil {
			return nil, err
		}
		res[i] = re
	}
	return res, nil
}

// hidePatternsForScope returns the patterns of the hide entries that apply to scope: entries
// without a "::" suffix apply everywhere, suffixed ones only to the listed scopes. Every hide-by-x
// setting shares this "pattern::scope1|scope2" syntax, only the pattern dialect differs.
func hidePatternsForScope(entries []string, scope string) []string {
	var patterns []string
	for _, entry := range entries {
		pattern, tags := splitHideEntry(entry)
		if len(tags) == 0 || slices.Contains(tags, scope) {
			patterns = append(patterns, pattern)
		}
	}
	return patterns
}

// splitHideEntry splits a "pattern::tag1|tag2" entry into its pattern and optional
// scope tags, e.g. "projects/archive::search|filter" -> ("projects/archive", ["search",
// "filter"]).
func splitHideEntry(entry string) (pattern string, tags []string) {
	pattern, tagStr, ok := strings.Cut(entry, "::")
	if !ok || tagStr == "" {
		return pattern, nil
	}
	return pattern, strings.Split(tagStr, "|")
}

func pathSegmentsMatch(patternSegs []*regexp.Regexp, candidateSegs []string) bool {
	for i, re := range patternSegs {
		if re != nil && !re.MatchString(candidateSegs[i]) {
			return false
		}
	}
	return true
}

// InitGitRepository initializes git repository based on configuration
func InitGitRepository() error {
	dataPath := appConfig.DataPath
	gitDir := filepath.Join(dataPath, ".git")

	if _, err := os.Stat(gitDir); !os.IsNotExist(err) {
		logging.LogInfo(logging.KeyApp, "git repository already exists in %s", dataPath)
		return nil
	}

	if appConfig.GitRemote != "" {
		var auth transport.AuthMethod
		if appConfig.GitSSHKey != "" {
			var err error
			auth, err = ssh.NewPublicKeysFromFile("git", appConfig.GitSSHKey, "")
			if err != nil {
				logging.LogError(logging.KeyApp, "failed to load ssh key for clone: %v", err)
				return err
			}
		}
		wt := osfs.New(dataPath)
		dot, err := utils.DotGitFilesystem(wt)
		if err != nil {
			logging.LogError(logging.KeyApp, "failed to clone repository: %v", err)
			return err
		}
		storer := filesystem.NewStorage(dot, cache.NewObjectLRUDefault())
		if _, err := git.Clone(storer, wt, &git.CloneOptions{
			URL:  appConfig.GitRemote,
			Auth: auth,
		}); err != nil {
			// git.Init (called by git.Clone before it fetches) already wrote a real .git dir to
			// disk, so a fetch/auth failure here would otherwise leave one behind - and the
			// early "already exists" check above would then treat that half-cloned repo as done
			// forever, blocking any retry. git.PlainClone avoids this itself (it deletes the
			// whole target dir on failure if it started empty); removing just gitDir is enough
			// here and, unlike PlainClone's approach, never risks deleting dataPath's other
			// existing content.
			if rmErr := os.RemoveAll(gitDir); rmErr != nil {
				// if this fails, the stat check above will keep treating the half-cloned
				// repo as done on every future call - surface it loudly rather than leaving
				// a silently stuck retry
				logging.LogError(logging.KeyApp, "failed to remove half-cloned repository at %s: %v", gitDir, rmErr)
			}
			logging.LogError(logging.KeyApp, "failed to clone repository: %v", err)
			return err
		}
		logging.LogInfo(logging.KeyApp, "git repository cloned from %s to %s", appConfig.GitRemote, dataPath)
	} else {
		wt := osfs.New(dataPath)
		dot, err := utils.DotGitFilesystem(wt)
		if err != nil {
			logging.LogError(logging.KeyApp, "failed to initialize git repository: %v", err)
			return err
		}
		storer := filesystem.NewStorage(dot, cache.NewObjectLRUDefault())
		if _, err := git.Init(storer, wt); err != nil {
			logging.LogError(logging.KeyApp, "failed to initialize git repository: %v", err)
			return err
		}
		logging.LogInfo(logging.KeyApp, "local git repository initialized in %s", dataPath)
	}

	return nil
}

// GetSearchEngine ..
func GetSearchEngine() string {
	return appConfig.SearchEngine
}

// loadEnvFile applies .env onto the process environment and reports what happened as a log
// line, rather than logging it directly: it runs before logging.Init (file logging needs
// KNOV_LOG_FILE_ENABLED etc., which may themselves come from .env), so logging it here would
// only reach stdout, not logs/app.log. The caller logs the returned message once file logging
// is up, so it lands in the file like every other startup line.
func loadEnvFile() (msg string, warn bool) {
	envPath := ".env"
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return "no .env file found, using environment variables and defaults", false
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		return fmt.Sprintf("failed to read .env file: %v", err), true
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

	return ".env file loaded", false
}

func ExtensionForEditor(editorType string) string {
	switch editorType {
	case "todo":
		return utils.Ternary(UseExtensionTodo.Get(), ".todo", ".md")
	case "list":
		return utils.Ternary(UseExtensionList.Get(), ".list", ".md")
	case "index":
		return utils.Ternary(UseExtensionIndex.Get(), ".index", ".md")
	case "tracker":
		return utils.Ternary(UseExtensionTracker.Get(), ".tracker", ".md")
	case "book":
		return utils.Ternary(UseExtensionBook.Get(), ".book", ".md")
	default:
		return ".md"
	}
}
