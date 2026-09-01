// Package configmanager - App configuration from environment variables
package configmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"knov/internal/backup"
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

// AppConfig contains environment-based application configuration. Each field's KNOV_*
// variable, default, description and parsing are declared exactly once, in envdefs.go's
// EnvVarDefs - not here. A field with no corresponding EnvVarDefs entry isn't 1:1 env-backed
// (e.g. LinkRegex is a fixed constant, not user-configurable).
type AppConfig struct {
	DataPath                    string
	ThemesPath                  string
	StoragePath                 string
	LogsPath                    string
	BackupsPath                 string
	ServerPort                  string
	GitRemote                   string
	GitRemoteBranch             string
	GitAutoPush                 bool
	GitPushTimeout              string
	GitUser                     string
	GitPassword                 string
	GitToken                    string
	GitSSHKey                   string
	ConfigStorageProvider       string
	MetadataStorageProvider     string
	CacheStorageProvider        string
	SearchStorageProvider       string
	KanbanEventsEnabled         bool
	KanbanEventsProvider        string
	SearchEngine                string
	LinkRegex                   []string
	CronjobInterval             string
	SearchIndexInterval         string
	MetadataRebuildInterval     string
	KanbanPrefix                string
	KanbanStatuses              []string
	KanbanColumns               []string
	AutoCreateTags              []AutoCreateTag
	KanbanTagColors             map[string]string
	KanbanCardStyles            map[string]string // status → "normal"|"italic"|"highlighted"|"deleted"
	KanbanArchiveStatus         string
	KanbanAncestorAllowedStatus []string
	KanbanBoards                []KanbanBoard
	NotifyDuration              int
	DefaultEditor               string
	BackupAutoProfiles          []BackupProfile
	BackupRotationKeepDays      int
	BackupRotationKeepDefault   int
	LogFileEnabled              bool
	LogMaxSizeMB                int
	LogMaxFiles                 int
	MOTD                        string
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

	if err := InitGitRepository(); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize git repository: %s", err)
	}

	logging.SetTimeFormatter(FormatDateTimeSeconds)
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

// GetKanbanAncestorAllowedStatus returns the statuses a descendant card must have for its
// ancestor to be listed in the ancestor filter; empty means no restriction (all allowed)
func GetKanbanAncestorAllowedStatus() []string {
	return appConfig.KanbanAncestorAllowedStatus
}

// GetKanbanBoards returns the configured folder-based kanban boards
func GetKanbanBoards() []KanbanBoard {
	return appConfig.KanbanBoards
}

// GetKanbanBoardBySlug looks up a configured kanban board by its URL slug
func GetKanbanBoardBySlug(slug string) (KanbanBoard, bool) {
	for _, b := range appConfig.KanbanBoards {
		if b.Slug == slug {
			return b, true
		}
	}
	return KanbanBoard{}, false
}

// GetKanbanBoardByFolder looks up a configured kanban board by its exact folder path
func GetKanbanBoardByFolder(folderPath string) (KanbanBoard, bool) {
	for _, b := range appConfig.KanbanBoards {
		if b.FolderPath == folderPath {
			return b, true
		}
	}
	return KanbanBoard{}, false
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

// getKanbanBoardsEnv parses "folder/path:Display Name, other/folder:Other Name" into a list of
// kanban boards, deriving a stable URL slug from each folder path (colliding slugs get a numeric
// suffix, same scheme as header-anchor IDs).
func getKanbanBoardsEnv(key string) []KanbanBoard {
	var boards []KanbanBoard
	usedSlugs := map[string]int{}
	value := os.Getenv(key)
	if value == "" {
		return boards
	}
	for _, pair := range strings.Split(value, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) != 2 {
			continue
		}
		folderPath := strings.Trim(strings.TrimSpace(parts[0]), "/")
		displayName := strings.TrimSpace(parts[1])
		if folderPath == "" || displayName == "" {
			continue
		}
		slug := utils.GenerateID(folderPath, usedSlugs)
		boards = append(boards, KanbanBoard{FolderPath: folderPath, DisplayName: displayName, Slug: slug})
	}
	return boards
}

// applyKanbanFolderSyncEnv marks the boards named in KNOV_KANBAN_FOLDERSYNC (a comma-separated
// list of board folder paths) as FolderSync-enabled. Kept separate from KNOV_KANBAN_BOARDS so the
// board list's free-text display names can never collide with a flag token.
func applyKanbanFolderSyncEnv(boards []KanbanBoard, key string) []KanbanBoard {
	for _, raw := range getStringListEnv(key, nil) {
		folderPath := strings.Trim(raw, "/")
		i := slices.IndexFunc(boards, func(b KanbanBoard) bool { return b.FolderPath == folderPath })
		if i == -1 {
			logging.LogWarning(logging.KeyApp, "%s names folder %q which has no matching board in KNOV_KANBAN_BOARDS, ignoring", key, folderPath)
			continue
		}
		boards[i].FolderSync = true
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
	return appConfig.KanbanStatuses
}

// GetKanbanColumns returns the visible kanban columns
func GetKanbanColumns() []string {
	return appConfig.KanbanColumns
}

// GetAutoCreateTags returns the folder-scoped tags applied to newly created files
func GetAutoCreateTags() []AutoCreateTag {
	return appConfig.AutoCreateTags
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
	case "index-editor":
		return HideIndex.Get()
	default:
		return false
	}
}

// Hide-path scopes: feature areas that can be targeted by a HidePaths entry's
// "::tag1|tag2" suffix (see IsPathHidden). Callers with no per-scope override
// (media) pass "" instead, which never matches a tag - an entry with tags can
// never hide a path from such a caller, only an entry with no suffix can.
const (
	HideScopeTree     = "tree"
	HideScopeBrowse   = "browse"
	HideScopeOverview = "overview"
	HideScopeSearch   = "search"
	HideScopeFilter   = "filter"
	HideScopeKanban   = "kanban"
)

// hideScopes lists every recognized HidePaths scope tag.
var hideScopes = []string{HideScopeTree, HideScopeBrowse, HideScopeOverview, HideScopeSearch, HideScopeFilter, HideScopeKanban}

// ValidateHidePaths rejects a HidePaths entry whose "::tag1|tag2" suffix contains an
// unrecognized scope name, so a typo (e.g. "::serach") fails on save instead of silently
// leaving the path hidden everywhere.
func ValidateHidePaths(entries []string) error {
	for _, entry := range entries {
		_, tags := splitHidePathEntry(entry)
		for _, tag := range tags {
			if !slices.Contains(hideScopes, tag) {
				return fmt.Errorf("hide paths: unknown scope %q in %q (known scopes: %s)", tag, entry, strings.Join(hideScopes, ", "))
			}
		}
	}
	return nil
}

// IsPathHidden checks if a relative folder path ("/"-separated, no leading/trailing slash)
// matches any of the configured hide-path patterns for the given scope.
//
// Each HidePaths entry is a pattern, optionally suffixed with "::tag1|tag2" (e.g.
// "projects/archive::search|filter"). An entry without a suffix always matches, keeping
// the path hidden everywhere. An entry with a suffix only matches (hides) when scope is
// listed in its tags - this is how a path can stay visible everywhere by default while
// being hidden in just the scopes it's tagged for, e.g. tagging it "filter" means it's
// hidden from filter results but stays visible everywhere else. Callers with no
// per-scope override (media, see FilterByVisibility) query with scope="", which never
// appears in a tag list - such a caller is never affected by a tagged entry, only by
// an entry with no suffix.
//
// Each pattern is itself "/"-separated; a pattern segment of "*" matches any single path
// segment, while any other segment is a case-insensitive regular expression that must fully
// match one segment. A pattern matches if its segments align with any contiguous run of the
// path's segments, e.g. "*/todo" hides every folder named "todo", while "test/todo" only
// hides the "todo" folder inside "test".
func IsPathHidden(relDirPath, scope string) bool {
	if relDirPath == "" {
		return false
	}
	pathSegs := strings.Split(relDirPath, "/")
	for _, entry := range HidePaths.Get() {
		pattern, tags := splitHidePathEntry(entry)
		if len(tags) > 0 && !slices.Contains(tags, scope) {
			continue
		}
		patternSegs := strings.Split(strings.Trim(pattern, "/"), "/")
		if len(patternSegs) == 0 || len(patternSegs) > len(pathSegs) {
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

// splitHidePathEntry splits a HidePaths entry into its pattern and optional scope tags,
// e.g. "projects/archive::search|filter" -> ("projects/archive", ["search", "filter"]).
func splitHidePathEntry(entry string) (pattern string, tags []string) {
	pattern, tagStr, ok := strings.Cut(entry, "::")
	if !ok || tagStr == "" {
		return pattern, nil
	}
	return pattern, strings.Split(tagStr, "|")
}

func pathSegmentsMatch(patternSegs, candidateSegs []string) bool {
	for i, seg := range patternSegs {
		if seg == "*" {
			continue
		}
		re, err := regexp.Compile("(?i)^" + seg + "$")
		if err != nil || !re.MatchString(candidateSegs[i]) {
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
		_, err := git.PlainClone(dataPath, false, &git.CloneOptions{
			URL:  appConfig.GitRemote,
			Auth: auth,
		})
		if err != nil {
			logging.LogError(logging.KeyApp, "failed to clone repository: %v", err)
			return err
		}
		logging.LogInfo(logging.KeyApp, "git repository cloned from %s to %s", appConfig.GitRemote, dataPath)
	} else {
		_, err := git.PlainInit(dataPath, false)
		if err != nil {
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
	default:
		return ".md"
	}
}
