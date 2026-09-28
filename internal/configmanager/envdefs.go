package configmanager

import (
	"path/filepath"
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
// constructors above: a composite AppConfig field (BackupAutoProfiles) or a
// var with no AppConfig field at all, read directly by another package (KNOV_LOG_*). apply may be nil for the latter case.
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

// EnvCategoryDescriptions documents a handful of categories (the Category argument shared by
// every entry in EnvVarDefs below) with a short blurb introducing the category as a whole,
// rather than any one var - shown above the category's section header in .env.example
// (tools/genenv) and reusable anywhere that wants the same intro (e.g. the "backup" entry on
// the /system/backup page). Not every category needs one; missing = no blurb.
var EnvCategoryDescriptions = map[string]string{
	"backup":  "Backups snapshot metadata, chat, kanban, notifications, config and search by default. Docs and media are optional - select them individually since they're already tracked by git, and including them can make a backup much larger. Each storage is backed up one at a time, not as a single point-in-time transaction. Backups can also run automatically on a schedule (KNOV_BACKUP_AUTO_PROFILES), on top of the manual backups triggered on /system/backup. Each profile is due-tracked off its own newest backup's timestamp rather than a \"ran today\" flag, so a device that isn't running 24/7 (e.g. a USB stick) still catches up reliably instead of missing a scheduled slot entirely.\n\n Rotation is tuned via KNOV_BACKUP_ROTATION_KEEP_DAYS (days of backups always kept) and KNOV_BACKUP_ROTATION_KEEP_DEFAULT (minimum number of default backups always kept, regardless of age). See .env.example for details.",
	"tracker": "Trackers are named counters (e.g. habit or hit-count tracking) with +/- buttons and a generated markdown stats table. Turning off KNOV_TRACKER_ENABLED hides the tracker editor and switches its day-delta storage to a noop backend - any existing tracker.db is left on disk untouched but reads as empty, so every tracker's stats markdown regenerates to all-zero (not deleted, just invisible) until it's turned back on.",
}

// EnvVarDefs lists every recognized KNOV_* environment variable, grouped and ordered the same
// way .env.example presents them.
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
	stringDef("KNOV_SERVER_HOST", "server", "interface the app listens on; empty = all interfaces, use 127.0.0.1 to only allow local access", "", func(c *AppConfig) *string { return &c.ServerHost }),
	stringDef("KNOV_MOTD", "server", "message shown in a banner at the top of every page (e.g. to label a test/copy environment); empty = hidden", "", func(c *AppConfig) *string { return &c.MOTD }),

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
	stringDef("KNOV_GIT_REMOTE", "git", "remote sync URL (leave empty for local-only mode); if set and no local repo exists yet, knov will clone it on first start", "", func(c *AppConfig) *string { return &c.GitRemote }),
	stringDef("KNOV_GIT_REMOTE_BRANCH", "git", "remote branch to sync with", "main", func(c *AppConfig) *string { return &c.GitRemoteBranch }),
	boolDef("KNOV_GIT_AUTO_PUSH", "git", "automatically push after every commit", true, func(c *AppConfig) *bool { return &c.GitAutoPush }),
	stringDef("KNOV_GIT_PUSH_TIMEOUT", "git", "timeout for push/pull operations", "10s", func(c *AppConfig) *string { return &c.GitPushTimeout }),
	stringDef("KNOV_GIT_USER", "git", "HTTPS username (leave empty for SSH or local repo)", "", func(c *AppConfig) *string { return &c.GitUser }),
	stringDef("KNOV_GIT_PASSWORD", "git", "HTTPS password (leave empty for SSH or local repo)", "", func(c *AppConfig) *string { return &c.GitPassword }, withSensitive()),
	stringDef("KNOV_GIT_TOKEN", "git", "HTTPS access token, takes priority over KNOV_GIT_PASSWORD (leave empty for SSH or local repo)", "", func(c *AppConfig) *string { return &c.GitToken }, withSensitive()),
	stringDef("KNOV_GIT_SSH_KEY", "git", "SSH private key path (leave empty to use ssh-agent or ~/.ssh/id_rsa, id_ed25519, id_ecdsa); set this if you use a non-default key file, e.g. ~/.ssh/id_rsa_privat", "", func(c *AppConfig) *string { return &c.GitSSHKey }),

	// ── storage providers ──
	stringDef("KNOV_CONFIG_STORAGE_PROVIDER", "storage providers", "app settings storage", "json", func(c *AppConfig) *string { return &c.ConfigStorageProvider }, withOptions("json")),
	stringDef("KNOV_METADATA_STORAGE_PROVIDER", "storage providers", "metadata storage", "sqlite", func(c *AppConfig) *string { return &c.MetadataStorageProvider }, withOptions("json", "yaml", "sqlite")),
	stringDef("KNOV_CACHE_STORAGE_PROVIDER", "storage providers", "cache storage", "sqlite", func(c *AppConfig) *string { return &c.CacheStorageProvider }, withOptions("json", "sqlite")),
	stringDef("KNOV_SEARCH_STORAGE_PROVIDER", "storage providers", "search index storage", "sqlite", func(c *AppConfig) *string { return &c.SearchStorageProvider }, withOptions("sqlite")),
	stringDef("KNOV_TRACKER_STORAGE_PROVIDER", "storage providers", "tracker counter day-delta storage", "sqlite", func(c *AppConfig) *string { return &c.TrackerStorageProvider }, withOptions("sqlite")),
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
	stringDef("KNOV_NOTIFY_MIN_LEVEL", "notifications", "minimum severity that still shows a toast (lower levels are still written to the notification log)", "info", func(c *AppConfig) *string { return &c.NotifyMinLevel }, withOptions("info", "warning", "error", "off")),

	// ── kanban ──
	boolDef("KNOV_KANBAN_EVENTS_ENABLED", "kanban", "set to false to disable kanban event logging entirely (no storage is created)", true, func(c *AppConfig) *bool { return &c.KanbanEventsEnabled }),
	stringDef("KNOV_KANBAN_PREFIX", "kanban", `prefix used for kanban tags (e.g. "kb" → tag: kb-status-inbox), letters, digits and _ only; existing tags are not renamed, so after a change cards still tagged with the old prefix drop off the board until they're retagged`, "kb", func(c *AppConfig) *string { return &c.KanbanPrefix }),

	// ── tracker ──
	boolDef("KNOV_TRACKER_ENABLED", "tracker", "set to false to disable the tracker editor entirely (no storage is created)", true, func(c *AppConfig) *bool { return &c.TrackerEnabled }),

	// ── backup ──
	compositeDef("KNOV_BACKUP_AUTO_PROFILES", "backup", `automatic backup profiles: semicolon-separated "<name>:<cron>:<storages>,<name>:<cron>:<storages>" and comma separated for multiple entries. name is freely chooseable; cron is a standard 5-field expression; storages is comma-separated and may be empty for the default backup set. Empty = automatic backups disabled entirely.

Examples:
    daily:0 0 * * *:                                     nightly backup of the default storage set
    weekly-docs:0 0 * * 0:docs,media                     weekly backup of docs and media, every Sunday at midnight
    daily:0 0 * * *:;weekly-docs:0 0 * * 0:docs,media    both combined into one profile list`, "",
		func(cfg *AppConfig, key string) { cfg.BackupAutoProfiles = getBackupProfilesEnv(key) },
		func(cfg AppConfig, _ string) string { return formatBackupProfiles(cfg.BackupAutoProfiles) },
	),
	intDef("KNOV_BACKUP_ROTATION_KEEP_DAYS", "backup", "how many days of backup sets (default or partial) to always keep, regardless of count", 7, func(c *AppConfig) *int { return &c.BackupRotationKeepDays }),
	intDef("KNOV_BACKUP_ROTATION_KEEP_DEFAULT", "backup", "on top of that, the minimum number of default backups to always keep regardless of age - a floor so coming back after months away still leaves something restorable. A backup set can also be locked individually on /system/backup to always be kept, ignoring both settings above, until unlocked", 10, func(c *AppConfig) *int { return &c.BackupRotationKeepDefault }),
	stringDef("KNOV_BACKUP_S3_BUCKET", "backup", "S3 bucket backup sets are written to and restored from. Non-empty switches backup storage from the local filesystem (KNOV_BACKUPS_PATH) to S3 (or any S3-compatible service: MinIO, Cloudflare R2, Backblaze B2, DigitalOcean Spaces). Leave empty for local-filesystem backups", "", func(c *AppConfig) *string { return &c.BackupS3Bucket }),
	stringDef("KNOV_BACKUP_S3_ENDPOINT", "backup", "S3 endpoint host, no scheme (e.g. s3.amazonaws.com, nyc3.digitaloceanspaces.com, localhost:9000). Required when KNOV_BACKUP_S3_BUCKET is set", "", func(c *AppConfig) *string { return &c.BackupS3Endpoint }),
	stringDef("KNOV_BACKUP_S3_REGION", "backup", "S3 region for backup storage (e.g. us-east-1); may be left empty for providers that don't need it", "", func(c *AppConfig) *string { return &c.BackupS3Region }),
	stringDef("KNOV_BACKUP_S3_PREFIX", "backup", "key prefix applied to every backup object in the bucket (e.g. knov/backups/); empty = bucket root", "", func(c *AppConfig) *string { return &c.BackupS3Prefix }),
	stringDef("KNOV_BACKUP_S3_ACCESS_KEY", "backup", "S3 access key id", "", func(c *AppConfig) *string { return &c.BackupS3AccessKey }, withSensitive()),
	stringDef("KNOV_BACKUP_S3_SECRET_KEY", "backup", "S3 secret access key", "", func(c *AppConfig) *string { return &c.BackupS3SecretKey }, withSensitive()),
	boolDef("KNOV_BACKUP_S3_USE_SSL", "backup", "connect to the S3 endpoint over HTTPS", true, func(c *AppConfig) *bool { return &c.BackupS3UseSSL }),
}

// applyEnvDefs populates cfg from every documented env var that has an AppConfig field
// (def.apply != nil), in EnvVarDefs order.
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

// EnvVarDescription looks up a KNOV_* key's documented description in EnvVarDefs, for callers
// (e.g. the /system/backup page) that want to show it inline instead of duplicating the text.
// Returns "" if undocumented.
func EnvVarDescription(key string) string {
	for _, def := range EnvVarDefs {
		if def.Key == key {
			return def.Description
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
