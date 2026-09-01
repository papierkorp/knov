package configmanager

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// EnvVarDef documents one KNOV_* environment variable and, via apply/get, is also the only
// place that reads/writes the AppConfig field it backs. A key is therefore typed exactly
// once: as the first argument to one of the constructors below (stringDef, pathDef, boolDef,
// intDef, listDef, compositeDef). This same definition drives InitAppConfig (apply),
// CurrentEnvValues (get - used by the /system/environment page and the admin panel, which
// both read through it instead of duplicating field lists), and the generated .env.example
// (tools/genenv). Edit an entry here and everything downstream follows.
type EnvVarDef struct {
	Key             string
	Category        string   // section heading, e.g. "paths", "server", "git"
	Description     string   // may contain "\n" for a multi-line comment/example
	AvailableValues []string // the fixed set of accepted values, if any; nil for free-form vars
	Default         string   // documented default value; "" = empty/unset by default
	Sensitive       bool     // true = redact the current value on the environment page/API

	apply func(cfg *AppConfig, baseDir string) // nil for vars with no AppConfig field (e.g. KNOV_LOG_*)
	get   func(cfg AppConfig) string
}

type defOption func(*EnvVarDef)

// withOptions documents the fixed set of accepted values for an env var (e.g. log level).
func withOptions(values ...string) defOption {
	return func(d *EnvVarDef) { d.AvailableValues = values }
}

// withSensitive marks an env var's current value as redacted on the environment page/API.
func withSensitive() defOption {
	return func(d *EnvVarDef) { d.Sensitive = true }
}

func applyOptions(d EnvVarDef, opts []defOption) EnvVarDef {
	for _, opt := range opts {
		opt(&d)
	}
	return d
}

// stringDef defines a string-typed AppConfig field.
func stringDef(key, category, description, def string, field func(*AppConfig) *string, opts ...defOption) EnvVarDef {
	return applyOptions(EnvVarDef{
		Key: key, Category: category, Description: description, Default: def,
		apply: func(cfg *AppConfig, _ string) { *field(cfg) = getEnv(key, def) },
		get:   func(cfg AppConfig) string { return *field(&cfg) },
	}, opts)
}

// pathDef defines a string-typed AppConfig field whose default is relative to the
// executable's directory (baseDir) rather than a fixed literal, e.g. KNOV_DATA_PATH.
func pathDef(key, category, description, def string, field func(*AppConfig) *string) EnvVarDef {
	return EnvVarDef{
		Key: key, Category: category, Description: description, Default: def,
		apply: func(cfg *AppConfig, baseDir string) { *field(cfg) = getEnv(key, filepath.Join(baseDir, def)) },
		get:   func(cfg AppConfig) string { return *field(&cfg) },
	}
}

// boolDef defines a bool-typed AppConfig field.
func boolDef(key, category, description string, def bool, field func(*AppConfig) *bool) EnvVarDef {
	return EnvVarDef{
		Key: key, Category: category, Description: description, Default: strconv.FormatBool(def),
		AvailableValues: []string{"true", "false"},
		apply:           func(cfg *AppConfig, _ string) { *field(cfg) = getBoolEnv(key, def) },
		get:             func(cfg AppConfig) string { return strconv.FormatBool(*field(&cfg)) },
	}
}

// intDef defines an int-typed AppConfig field.
func intDef(key, category, description string, def int, field func(*AppConfig) *int) EnvVarDef {
	return EnvVarDef{
		Key: key, Category: category, Description: description, Default: strconv.Itoa(def),
		apply: func(cfg *AppConfig, _ string) { *field(cfg) = getIntEnv(key, def) },
		get:   func(cfg AppConfig) string { return strconv.Itoa(*field(&cfg)) },
	}
}

// listDef defines a []string-typed AppConfig field, stored/read as a comma-separated list.
func listDef(key, category, description string, def []string, field func(*AppConfig) *[]string) EnvVarDef {
	return EnvVarDef{
		Key: key, Category: category, Description: description, Default: strings.Join(def, ","),
		apply: func(cfg *AppConfig, _ string) { *field(cfg) = getStringListEnv(key, def) },
		get:   func(cfg AppConfig) string { return strings.Join(*field(&cfg), ", ") },
	}
}

// compositeDef defines an env var whose parsing/formatting doesn't fit the type-based
// constructors above: a composite AppConfig field (KanbanBoards, AutoCreateTags, the
// tag-color/card-style maps) or a var with no AppConfig field at all, read directly by
// another package (KNOV_LOG_*). apply may be nil for the latter case.
func compositeDef(key, category, description, def string, apply func(cfg *AppConfig, key string), get func(cfg AppConfig, key string) string, opts ...defOption) EnvVarDef {
	return applyOptions(EnvVarDef{
		Key: key, Category: category, Description: description, Default: def,
		apply: func(cfg *AppConfig, _ string) {
			if apply != nil {
				apply(cfg, key)
			}
		},
		get: func(cfg AppConfig) string { return get(cfg, key) },
	}, opts)
}

// getRaw reads a key straight from the process environment - the get for compositeDef entries
// with no AppConfig field, or whose current value is best shown as-is (KNOV_KANBAN_FOLDERSYNC).
func getRaw(_ AppConfig, key string) string {
	return os.Getenv(key)
}

// EnvVarDefs lists every recognized KNOV_* environment variable, grouped and ordered the same
// way .env.example presents them. KNOV_KANBAN_FOLDERSYNC must stay after KNOV_KANBAN_BOARDS:
// its apply flags an already-parsed board by folder path, so KanbanBoards needs to exist first.
var EnvVarDefs = []EnvVarDef{
	// ── paths ── Default here is relative to the executable's directory (pathDef joins it
	// onto that directory), matching the Description on each.
	pathDef("KNOV_DATA_PATH", "paths", "path to the markdown data folder (relative to the executable)", "data", func(c *AppConfig) *string { return &c.DataPath }),
	pathDef("KNOV_THEMES_PATH", "paths", "path to the themes folder (relative to the executable)", "themes", func(c *AppConfig) *string { return &c.ThemesPath }),
	pathDef("KNOV_STORAGE_PATH", "paths", "path to the app's internal storage (metadata/cache/search/kanban databases; relative to the executable)", "storage", func(c *AppConfig) *string { return &c.StoragePath }),
	pathDef("KNOV_LOGS_PATH", "paths", "path to the log files (relative to the executable)", "logs", func(c *AppConfig) *string { return &c.LogsPath }),
	pathDef("KNOV_BACKUPS_PATH", "paths", "path backup sets are written to and restored from (relative to the executable)", "backups", func(c *AppConfig) *string { return &c.BackupsPath }),

	// ── server ──
	stringDef("KNOV_SERVER_PORT", "server", "port the app listens on", "1324", func(c *AppConfig) *string { return &c.ServerPort }),
	stringDef("KNOV_MOTD", "server", "message shown in a banner at the top of every page (e.g. to label a test/copy\nenvironment); empty = hidden", "", func(c *AppConfig) *string { return &c.MOTD }),

	// ── logging ── KNOV_LOG_LEVEL and KNOV_LOG_FILE_LEVEL have no AppConfig field: they're read
	// directly via os.Getenv in internal/logging on every log call (their get below just reuses
	// getEnv for the same "info" fallback, for display), so a level change takes effect without
	// a restart. The other three back real AppConfig fields, consumed once at startup by
	// logging.Init.
	compositeDef("KNOV_LOG_LEVEL", "logging", "stdout log level", "info", nil,
		func(_ AppConfig, key string) string { return getEnv(key, "info") }, withOptions("debug", "info", "warning", "error")),
	boolDef("KNOV_LOG_FILE_ENABLED", "logging", "file logging (writes to logs/app.log with rotation)", true, func(c *AppConfig) *bool { return &c.LogFileEnabled }),
	compositeDef("KNOV_LOG_FILE_LEVEL", "logging", "file log level", "info", nil,
		func(_ AppConfig, key string) string { return getEnv(key, "info") }, withOptions("debug", "info", "warning", "error")),
	intDef("KNOV_LOG_MAX_SIZE_MB", "logging", "max size in MB before rotating", 10, func(c *AppConfig) *int { return &c.LogMaxSizeMB }),
	intDef("KNOV_LOG_MAX_FILES", "logging", "number of rotated files to keep", 5, func(c *AppConfig) *int { return &c.LogMaxFiles }),

	// ── git ──
	stringDef("KNOV_GIT_REMOTE", "git", "remote sync URL (leave empty for local-only mode); if set and no local\nrepo exists yet, knov will clone it on first start", "", func(c *AppConfig) *string { return &c.GitRemote }),
	stringDef("KNOV_GIT_REMOTE_BRANCH", "git", "remote branch to sync with", "main", func(c *AppConfig) *string { return &c.GitRemoteBranch }),
	boolDef("KNOV_GIT_AUTO_PUSH", "git", "automatically push after every commit", true, func(c *AppConfig) *bool { return &c.GitAutoPush }),
	stringDef("KNOV_GIT_PUSH_TIMEOUT", "git", "timeout for push/pull operations", "10s", func(c *AppConfig) *string { return &c.GitPushTimeout }),
	stringDef("KNOV_GIT_USER", "git", "HTTPS username (leave empty for SSH or local repo)", "", func(c *AppConfig) *string { return &c.GitUser }),
	stringDef("KNOV_GIT_PASSWORD", "git", "HTTPS password (leave empty for SSH or local repo)", "", func(c *AppConfig) *string { return &c.GitPassword }, withSensitive()),
	stringDef("KNOV_GIT_TOKEN", "git", "HTTPS access token, takes priority over KNOV_GIT_PASSWORD (leave empty for SSH or local repo)", "", func(c *AppConfig) *string { return &c.GitToken }, withSensitive()),
	stringDef("KNOV_GIT_SSH_KEY", "git", "SSH private key path (leave empty to use ssh-agent or ~/.ssh/id_rsa, id_ed25519, id_ecdsa);\nset this if you use a non-default key file, e.g. ~/.ssh/id_rsa_privat", "", func(c *AppConfig) *string { return &c.GitSSHKey }),

	// ── storage providers ──
	stringDef("KNOV_CONFIG_STORAGE_PROVIDER", "storage providers", "app settings storage", "json", func(c *AppConfig) *string { return &c.ConfigStorageProvider }, withOptions("json")),
	stringDef("KNOV_METADATA_STORAGE_PROVIDER", "storage providers", "metadata storage", "sqlite", func(c *AppConfig) *string { return &c.MetadataStorageProvider }, withOptions("json", "yaml", "sqlite")),
	stringDef("KNOV_CACHE_STORAGE_PROVIDER", "storage providers", "cache storage", "sqlite", func(c *AppConfig) *string { return &c.CacheStorageProvider }, withOptions("json", "sqlite")),
	stringDef("KNOV_SEARCH_STORAGE_PROVIDER", "storage providers", "search index storage", "sqlite", func(c *AppConfig) *string { return &c.SearchStorageProvider }, withOptions("sqlite")),
	stringDef("KNOV_KANBAN_EVENTS_STORAGE_PROVIDER", "storage providers", "kanban event log storage", "sqlite", func(c *AppConfig) *string { return &c.KanbanEventsProvider }, withOptions("json", "sqlite")),

	// ── search ──
	stringDef("KNOV_SEARCH_ENGINE", "search", "search engine", "repository", func(c *AppConfig) *string { return &c.SearchEngine }, withOptions("repository", "grep")),

	// ── cronjob ──
	stringDef("KNOV_CRONJOB_INTERVAL", "cronjob", "how often the background cronjob runs (file-sync, kanban folder-sync, ...)", "5m", func(c *AppConfig) *string { return &c.CronjobInterval }),
	stringDef("KNOV_SEARCH_INDEX_INTERVAL", "cronjob", "how often the search index is rebuilt", "15m", func(c *AppConfig) *string { return &c.SearchIndexInterval }),
	stringDef("KNOV_METADATA_REBUILD_INTERVAL", "cronjob", "how often metadata is rebuilt from disk", "60m", func(c *AppConfig) *string { return &c.MetadataRebuildInterval }),

	// ── editor ──
	stringDef("KNOV_DEFAULT_EDITOR", "editor", "default editor for new and unassigned markdown files (empty = use user setting)", "", func(c *AppConfig) *string { return &c.DefaultEditor }, withOptions("codemirror-editor")),

	// ── notifications ──
	intDef("KNOV_NOTIFY_DURATION", "notifications", "how long toast notifications stay visible, in milliseconds", 3500, func(c *AppConfig) *int { return &c.NotifyDuration }),

	// ── kanban ──
	boolDef("KNOV_KANBAN_EVENTS_ENABLED", "kanban", "set to false to disable kanban event logging entirely (no storage is created)", true, func(c *AppConfig) *bool { return &c.KanbanEventsEnabled }),
	stringDef("KNOV_KANBAN_PREFIX", "kanban", `prefix used for kanban tags (e.g. "kb" → tag: kb-status-inbox)`, "kb", func(c *AppConfig) *string { return &c.KanbanPrefix }),
	listDef("KNOV_KANBAN_STATUS", "kanban", "all possible kanban statuses (comma-separated, defines all valid tag values)", []string{"inbox", "inprogress", "blocked", "archive"}, func(c *AppConfig) *[]string { return &c.KanbanStatuses }),
	listDef("KNOV_KANBAN_COLUMNS", "kanban", "visible columns on the board (subset of KNOV_KANBAN_STATUS)", []string{"inbox", "inprogress", "blocked"}, func(c *AppConfig) *[]string { return &c.KanbanColumns }),
	compositeDef("KNOV_AUTOCREATE_TAGS", "kanban", "tags automatically added to newly created files (comma-separated, empty = disabled).\na bare tag (no \":\") applies to every new file everywhere; a \"folder/path:tag\" entry only\napplies to files created under that folder (recursive - also covers subfolders)\ne.g. starred, projects/work:kb-status-inbox, personal/todo:kb-status-inbox", "",
		func(cfg *AppConfig, key string) { cfg.AutoCreateTags = getAutoCreateTagsEnv(key) },
		func(cfg AppConfig, _ string) string { return formatAutoCreateTags(cfg.AutoCreateTags) },
	),
	compositeDef("KNOV_KANBAN_TAG_COLORS", "kanban", "custom css colors for specific tags on the kanban board (tag:csscolor, comma-separated)\ne.g. username:green,urgent:red,blocked:orange", "",
		func(cfg *AppConfig, key string) { cfg.KanbanTagColors = getStringMapEnv(key) },
		func(cfg AppConfig, _ string) string { return formatStringMap(cfg.KanbanTagColors) },
	),
	compositeDef("KNOV_KANBAN_CARD_STYLES", "kanban", "card style per kanban status (status:style, comma-separated); styles: normal, italic, highlighted, deleted\ne.g. blocked:italic,waiting:italic,urgent:highlighted,done:deleted,archive:deleted", "",
		func(cfg *AppConfig, key string) { cfg.KanbanCardStyles = getStringMapEnv(key) },
		func(cfg AppConfig, _ string) string { return formatStringMap(cfg.KanbanCardStyles) },
	),
	stringDef("KNOV_KANBAN_ARCHIVE_STATUS", "kanban", "status used for the archive drop zone shown while dragging (empty = disable the archive zone)", "archive", func(c *AppConfig) *string { return &c.KanbanArchiveStatus }),
	listDef("KNOV_KANBAN_ANCESTOR_ALLOWED_STATUS", "kanban", "statuses a descendant card must have for its ancestor to appear in the ancestor filter\n(comma-separated, subset of KNOV_KANBAN_STATUS; empty = no restriction, all ancestors shown)\ne.g. inbox,inprogress,blocked", nil, func(c *AppConfig) *[]string { return &c.KanbanAncestorAllowedStatus }),
	compositeDef("KNOV_KANBAN_BOARDS", "kanban", "kanban boards (folder/path:Display Name, comma-separated); each board covers that folder and\nits subfolders. The URL slug is derived from the folder path automatically.\ne.g. projects/work:Work Board,personal/todo:Personal Todo", "",
		func(cfg *AppConfig, key string) { cfg.KanbanBoards = getKanbanBoardsEnv(key) },
		func(cfg AppConfig, _ string) string { return formatKanbanBoards(cfg.KanbanBoards) },
	),
	compositeDef("KNOV_KANBAN_FOLDERSYNC", "kanban", "board folder paths (comma-separated, subset of KNOV_KANBAN_BOARDS) that enable foldersync:\nmoving a card physically moves the file into folder/path/<status>/, and moving the file on disk\ninto an existing folder/path/<status>/ folder sets the tag (picked up by the file-sync cronjob).\nif you move files by hand outside the app, trigger a manual file-sync run before dragging cards\nfor those files in the UI - the board doesn't know about an external move until the cronjob has\nrun, so a drag against a file that was already moved on disk will fail against its stale path.\ne.g. projects/work", "",
		func(cfg *AppConfig, key string) { cfg.KanbanBoards = applyKanbanFolderSyncEnv(cfg.KanbanBoards, key) },
		getRaw,
	),

	// ── backup ──
	compositeDef("KNOV_BACKUP_AUTO_PROFILES", "backup", "automatic backup profiles (semicolon-separated; name:cron:storages per profile, on top of manually\ntriggered backups on /system/backup); cron is a standard 5-field expression (minute hour\nday-of-month month day-of-week, e.g. \"0 18 * * *\" for daily at 18:00); storages is comma-separated\nand may be empty for the default backup set. Empty = automatic backups disabled entirely. Each\nprofile is due-tracked off its own newest backup's timestamp, not a \"ran today\" flag, so a device\nthat isn't running 24/7 (e.g. a USB stick) still catches up reliably: it backs up as soon as it's\nnext on past a missed occurrence, instead of a slot that only ever lands outside its usage window\ngetting skipped entirely\ne.g. daily:0 0 * * *:metadata,chat,kanban,notifications,config;weekly-docs:0 0 * * 0:docs,media", "",
		func(cfg *AppConfig, key string) { cfg.BackupAutoProfiles = getBackupProfilesEnv(key) },
		func(cfg AppConfig, _ string) string { return formatBackupProfiles(cfg.BackupAutoProfiles) },
	),
	intDef("KNOV_BACKUP_ROTATION_KEEP_DAYS", "backup", "how many days of backup sets (default or partial) to always keep, regardless of count", 7, func(c *AppConfig) *int { return &c.BackupRotationKeepDays }),
	intDef("KNOV_BACKUP_ROTATION_KEEP_DEFAULT", "backup", "on top of that, the minimum number of default backups to always keep regardless of age - a floor\nso coming back after months away still leaves something restorable. A backup set can also be\nlocked individually on /system/backup to always be kept, ignoring both settings above, until unlocked", 10, func(c *AppConfig) *int { return &c.BackupRotationKeepDefault }),
}

// applyEnvDefs populates cfg from every documented env var that has an AppConfig field
// (def.apply != nil), in EnvVarDefs order - the order matters for KNOV_KANBAN_BOARDS /
// KNOV_KANBAN_FOLDERSYNC (see EnvVarDefs doc comment).
func applyEnvDefs(cfg *AppConfig, baseDir string) {
	for _, def := range EnvVarDefs {
		if def.apply != nil {
			def.apply(cfg, baseDir)
		}
	}
}

// envVarDefault looks up a KNOV_* key's documented default in EnvVarDefs, for the handful of
// call sites (initLogLevel) that need it outside of applyEnvDefs. Returns "" if undocumented.
func envVarDefault(key string) string {
	for _, def := range EnvVarDefs {
		if def.Key == key {
			return def.Default
		}
	}
	return ""
}

// CurrentEnvValues returns the effective current value for every documented env var
// (EnvVarDefs): each entry's own get closure reads the live value, whether that's an
// AppConfig field or, for vars with no AppConfig field, os.Getenv directly.
func CurrentEnvValues() map[string]string {
	values := make(map[string]string, len(EnvVarDefs))
	for _, def := range EnvVarDefs {
		values[def.Key] = def.get(appConfig)
	}
	return values
}

func formatStringMap(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+":"+m[k])
	}
	return strings.Join(parts, ", ")
}

func formatKanbanBoards(boards []KanbanBoard) string {
	parts := make([]string, 0, len(boards))
	for _, b := range boards {
		parts = append(parts, b.FolderPath+":"+b.DisplayName)
	}
	return strings.Join(parts, ", ")
}

func formatAutoCreateTags(tags []AutoCreateTag) string {
	parts := make([]string, 0, len(tags))
	for _, a := range tags {
		if a.FolderPath != "" {
			parts = append(parts, a.FolderPath+":"+a.Tag)
		} else {
			parts = append(parts, a.Tag)
		}
	}
	return strings.Join(parts, ", ")
}
