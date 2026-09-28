# Developer Guide

**Prerequisites**

- Go 1.21 or later
- Git
- Make
- Swag CLI + gotext: `make install-tools` (versions pinned in the root `Makefile`, matching what's already in `go.mod`)

**Extra prerequisites for the Android wrapper (`android/`)**

- Android Studio (bundles JDK, Gradle, and the Android SDK), or a standalone JDK 17 + Android SDK + Gradle install
- Android SDK Platform 34 and Build-Tools (installed via Android Studio's SDK Manager)
- A device/emulator with "install unknown apps" enabled for sideloading (this isn't published to the Play Store)
- `make mobile-apk` needs `ANDROID_JAVA_HOME` pointed at a JDK Gradle/this AGP version understands - the system `JAVA_HOME` may be too new. If using Android Studio, its bundled JBR works: find it under Android Studio's install dir (e.g. `.../android-studio/jbr`, or `.../extra/jbr` for a Flatpak install) and `export ANDROID_JAVA_HOME=/path/to/jbr`. Falls back to `JAVA_HOME` if `ANDROID_JAVA_HOME` isn't set - not a concern for `make docker-build-apk`/CI, which build inside `tools/docker_android/Dockerfile` and don't use `mobile-apk`'s JDK detection at all, that image pins its own `JAVA_HOME`
- See `android/README.md` for build steps
- Can't install the SDK locally? `make docker-build-apk` builds `android/app/build/outputs/apk/debug/app-debug.apk` inside a container instead (see `tools/docker_android/Dockerfile`) - it builds the `docker-prepare-android-image` image first if it isn't already there. Run `make docker-prepare-android-image` on its own if you only want to (re)build the image, e.g. after a Dockerfile change

**Git on Android**

- Android's app-sandbox rejects the `flock(2)` syscall with `ENOSYS` ("function not implemented"). go-git calls it on every ref write (`HEAD` on init, the branch ref on every commit), so on Android, plain `git.PlainInit`/`PlainOpen`/`PlainClone` crash git init immediately, and every commit after it
- Fixed by building the same filesystem/storage go-git builds internally by hand - via `git.Init`/`git.Open`/`git.Clone`, the lower-level functions `PlainInit`/`PlainOpen`/`PlainClone` wrap - and, only when `runtime.GOOS == "android"`, wrapping the `.git` filesystem in a type that reports no read+write capability. That makes go-git's own dotgit package take its existing lock-free ref-write path (`setRefNorwfs`) instead of the locking one (`setRefRwfs`, which calls `flock`) - not a hack, go-git ships and tests this path itself for filesystems that can't lock
- The wrapper (`dotGitFilesystem` + `noRWFilesystem`) is duplicated in `internal/configmanager/config.go`'s `InitGitRepository` (both the local-init and remote-clone branches) and `internal/git/git.go`'s `openRepo` (used by every commit) - can't be shared between them since `internal/git` already imports `configmanager`, so the reverse import would cycle. `internal/test/testdata.go`'s test-data-seeding commit reuses the fix instead via `internal/git`'s exported `OpenRepository()`, since no cycle blocks that one
- Gated by `runtime.GOOS`, not applied everywhere: skipping the OS lock trades away protection against a *different* process (a manual `git commit`, another git GUI) racing the same repo at the same time - a real, if narrow, risk that Linux/Windows don't need to take on since `flock` works fine there

## Updating the Go Version

1. Check the official Go release notes before bumping — they list any breaking changes.
2. Install the new Go version and set it as active.
3. Update `go.mod`: change the `go` directive to the new version.
4. Run `go mod tidy` — this syncs toolchain requirements and may add/update a `toolchain` line.

## Updating Dependencies

### Font Awesome

```bash
cd static
rm font-awesome-<old-version>-all.min.css

cd font-awesome
mkdir webfonts/<new-version>
mkdir otfs/<new-version>

cd ~
git clone git@github.com:FortAwesome/Font-Awesome.git
cd Font-Awesome
cp webfonts/* .../static/font-awesome/webfonts/<new-version>
cp all.min.css .../static/font-awesome-<new-version>-all.min.css

cd otfs
pip install "fonttools[repacker]" otf2ttf
otf2ttf "Font Awesome 7 Free-Solid-900.otf"
cp *.ttf .../static/font-awesome/otfs/<new-version>
```

then update:

- the link in `base.gohtml` from `<link rel="stylesheet" href="/static/font-awesome-<old-version>-all.min.css"/>` to `<link rel="stylesheet" href="/static/font-awesome-<new-version>-all.min.css"/>`
- the webfonts path in `server.go` in the `handleWebfontsRedirect` function


## Quick Start

Clone and setup:

```bash
git clone https://github.com/papierkorp/knov.git
cd knov
go mod download
```

Install required tools:

```bash
make install-tools
```

Start development server:

```bash
# Start development server
make dev
make dev-fast # without fts5 search init

# Generate Swagger docs
make swaggo-api-init

# Generate translations
make translation

# Build for production
make prod
```

## Docker

Two Dockerfiles under `tools/`, for different purposes:

- `tools/docker_dev/` — local dev without installing Go/swag/gotext; `make devd` builds the image and runs it with the repo bind-mounted, doing the same as `make dev`
- `tools/docker_deployment/` — minimal production image (multi-stage build, no dev tooling); `make docker-build-deployment` builds it, or use the `docker-compose.yml` in that folder to run it with persistent volumes
- Pushing images to a registry isn't wired up yet — built and run locally for now

## API Development

### Adding New Endpoints

1. Add handler function to appropriate `internal/server/api_*.go` file
2. Add route in `internal/server/server.go`
3. Add Swagger documentation comments

## Translation

Add translatable strings in templates:

```go
{{T "Your translatable text"}}
```

Add translatable strings in Go code (global):

```go
translation.Sprintf("Your translatable text")
```

Add translatable strings in HTMX handlers (user-specific):

```go
func handleSomeHTMX(w http.ResponseWriter, r *http.Request) {
    // Use user's current language setting
    userLang := configmanager.GetLanguage()
    text := translation.SprintfForRequest(userLang, "Your translatable text")
    html := fmt.Sprintf(`<div>%s</div>`, text)
    w.Write([]byte(html))
}
```

Add translatable strings in a `render` package that already threads a per-request `lang` via a local `t` closure:

```go
t := func(key string, args ...any) string { return translation.SprintfForRequest(lang, key, args...) }
text := t("Your translatable text")
```

Add translatable strings on setting definitions (`Label`, `Desc`, `Description` struct fields):

```go
Label: "Vim Keybindings",
Desc:  "enable vim normal / insert / visual mode in the code editor",
```

Generate translations:

```bash
make translation
```

This runs `tools/i18nextract`, which parses the Go AST and templates for the four patterns above and refreshes the catalog. It requires the `gotext` CLI to be installed.

Translation files in `internal/translation/locales/{lang}/messages.gotext.json`

## Embedded Assets

### Static Files

Static files are embedded from the project root:

```go
//go:embed static/*
var staticFS embed.FS
```

### Theme Assets

Builtin theme assets are embedded in main.go:

```go
//go:embed themes/builtin
var builtinThemeFS embed.FS
```

Plugin themes embed their own assets:

```go
//go:embed templates/*.css
var cssFiles embed.FS
```

## Configuration Management

Two layers:

| Layer | Source | Requires restart |
|---|---|---|
| **AppConfig** | Environment variables | Yes |
| **Settings** | `storage/config/settings.json` | No |

**AppConfig** (`config.go`) — server-level options (paths, ports, intervals) loaded once at startup. Each `KNOV_*` var is declared exactly once as an `EnvVarDef` in `envdefs.go` (key, category, description, default, and how it applies to/reads from `AppConfig`) — that single definition drives `InitAppConfig`, the `/system/environment` page/API, and the generated `.env.example` (`make env-example`, via `tools/genenv`). Add a field to `AppConfig` and a matching `EnvVarDef` entry; nothing else needs touching.

**Settings** (`settings_*.go`) — user preferences editable in the UI. Each setting is a typed package-level variable declared in `settings_registry.go` and registered at init time. Adding a setting there is all that's needed — persistence, UI rendering, and `MyNewSetting.Get()` access are automatic.

- Types: `*BoolSetting`, `*IntSetting`, `*StringSetting` (also renders as select or dynamic-select), `*StringSliceSetting`, `*MapSetting[T]` (structured/nested values, no UI)
- Sections and groups are declared in `settings_definitions.go` and appear in the UI automatically
- `OnChange` fires whenever a value is stored - on saves (`SetSetting`, `BulkSetFromForm`, `ImportSettingsJSON`) and on the startup load in `InitSettings`
- `IntSetting` and `StringSetting` validate against `Min`/`Max` and `Options` respectively; invalid API values return 400, invalid stored values fall back to the default with a warning
- All values are stored in `atomic.Pointer[T]` — reads are lock-free. `MapSetting` uses copy-on-write: always build a fresh copy before calling `Set`, never mutate the map returned by `Get`

## Filter File - System

**Storage**

- Filter configs are stored in configStorage (JSON) under the key `filter/<filterID>`
- The filter ID is a unique path-like string, e.g. `my/notes-filter`

**Paired Index File**

- Every filter has a paired physical index file in `data/docs/`
- Path: `<filterID>` + extension from the `useExtensionIndex` UI setting (`.index` or `.md`)
- Example: filter `my/notes-filter` → `data/docs/my/notes-filter.index`
- Content: a markdown link list of all files matching the filter at last run, e.g. `- [path](path)`
- Metadata is saved with `Editor: filter-editor` so the filter editor opens when viewing the file

**Lifecycle**

- Save filter → config written to configStorage + index file generated immediately
- Delete filter → index file deleted + its metadata deleted + config removed from configStorage
- Cronjob → regenerates all filter index files on every file job interval (keeps results fresh)

**Viewing & Editing**

- Navigate to `/files/<filterID>.index` to view/edit the filter
- The filter editor opens (not the index editor) because metadata marks the file as `filter-editor`
- The index file content is always overwritten on save/cronjob — manual edits are lost

# Logging

Centralised logging in `internal/logging`. All app code uses the four level functions (`LogDebug`, `LogInfo`, `LogWarning`, `LogError`) — never the standard `log` package directly. Every call takes a `Key` as its first argument.

**Keys**

- `Key` identifies the destination. `KeyApp` (the zero value) is the general log, written to `logs/app.log`. Every other key gets its own rotating file at `logs/<key>.log` — never duplicated into `app.log`.
- Valid keys are declared as constants in `internal/logging` and listed in `AvailableKeys` (powers the admin log viewer's source filter).
- Adding a new key: add the const + add it to `AvailableKeys`. Only give a shared function its own key when it's genuinely noisy *and* its callers disagree on attribution — a function called from exactly one place should just hardcode that caller's key directly, no threading needed.

**Ring buffer**

- Every log entry (any key) is held in a fixed-size in-memory ring buffer (last 500 entries), regardless of level.
- Powers the in-app log viewer at `/system/logs` — live-polled via htmx, filterable by level/source/text, pauseable.

**File output**

- Opt-in via `KNOV_LOG_FILE_ENABLED` (default on). `KeyApp` writes to `logs/app.log`; every other key gets its own rotating file, all sharing the same rotation settings.
- Level threshold controlled by `KNOV_LOG_FILE_LEVEL` (default `info`), applied the same regardless of key.
- A session separator is written on startup (and on each scheduled-job run, for job keys), so restarts/runs are immediately visible when scrolling — the log viewer collapses these into expandable sections.

**Standard library interception**

- `InitInterceptor()` wraps `log.SetOutput` so third-party/framework `log.Printf` calls (e.g. chi access logs) are also captured into the ring buffer and `app.log`. Must be called before any other initialisation to avoid missing early entries.
- Console output from the four level functions bypasses this interceptor (writes straight to stdout) — they already record their own ring buffer entry and file line directly, so routing through the interceptor too would double-log everything back into `app.log`.

**Env vars**

```
KNOV_LOG_LEVEL          # console threshold: debug | info | warning | error (default: info)
KNOV_LOG_FILE_ENABLED   # set to "false" to disable file output (default: on)
KNOV_LOG_FILE_LEVEL     # file threshold: debug | info | warning | error (default: info)
KNOV_LOG_MAX_SIZE_MB    # max size per log file in MB before rotation (default: 10)
KNOV_LOG_MAX_FILES      # number of rotated files to keep (default: 5)
KNOV_LOGS_PATH          # override the logs directory (default: ./logs)
```

# Concurrent Writes (keylock)

- `internal/keylock` hands out one mutex per string key, lazily created and never freed. It's the shared fix for a lost-update race: two goroutines that read the same keyed record, each compute a change, and save back independently can otherwise silently revert one another.
- Used by `internal/files` (metadata, keyed by file path), `internal/kanban` (card order, keyed by board folder), and `internal/dashboard` (keyed by dashboard id) — each package owns a private `keylock.Registry` and exposes its own `Mutate`-style entry point (load under the key's lock, let a callback modify the record, save before unlocking).
- The raw get/save primitives underneath are unexported or documented as whole-record-replace-only, so a get-then-save outside the lock isn't something you can do by accident — going through the package's `Mutate` helper is the only path for a partial update.
- Locks aren't reentrant and are scoped to one key at a time — never lock a second key while already holding one; defer any cross-key write as a closure run after the first lock is released, otherwise two goroutines doing the mirror-image update can deadlock each other.
- Any new keyed record with concurrent writers (another per-id JSON blob, etc.) should follow the same shape rather than a bespoke lock.

# dbmigration

Tiny version-based schema migrations for sqlite. No external tools, no SQL files — migrations are plain Go functions.

## How it works

- A `schema_version` table holds a single integer: the db's current version.
- Each storage keeps its own ordered `[]Migration` and a `schemaVersion` const.
- On startup, `Migrate` compares the stored version to the target and runs the gap: missing `Up` steps when behind, `Down` steps in reverse when ahead.
- Each step plus its version bump run in one transaction — a failure rolls back cleanly and the version never lands in a half-applied state.

Version `N` is reached by applying `migrations[0..N-1]`. The slice index is the "from" version: `migrations[0]` is 0→1, `migrations[1]` is 1→2, etc.

## Usage

```go
const schemaVersion = 2

var migrations = []dbmigration.Migration{
    {Up: migrateV0toV1, Down: migrateV1toV0},
    {Up: migrateV1toV2, Down: migrateV2toV1},
}

func (ss *sqliteStorage) initialize() error {
    return dbmigration.Migrate(ss.db, schemaVersion, migrations)
}

func migrateV1toV2(tx *sql.Tx) error {
    _, err := tx.Exec(`ALTER TABLE metadata ADD COLUMN related TEXT`)
    return err
}

func migrateV2toV1(tx *sql.Tx) error {
    _, err := tx.Exec(`ALTER TABLE metadata DROP COLUMN related`)
    return err
}
```

## Rules

- Migrations are **append-only**. Never edit a shipped step — add a new one.
- Each storage owns its own version counter; they advance independently.
- Always bump `schemaVersion` when appending a migration.
- `Down` may be `nil` for irreversible steps; downgrading past one returns an error.
- Dropping a column requires sqlite ≥ 3.35 (2021). For older sqlite, use the create-new/copy/rename pattern.

## Migration test

**setup test folder**

```bash
mkdir /home/markus/develop/privat/migration-test-knov
cd /home/markus/develop/privat/migration-test-knov
cp /home/markus/develop/privat/knov/bin/knov .
./knov  # start once to let it initialize all sqlite DBs, then stop it
```

**change metadataStorage_sqlite.go**

Either use functions or put the migration directly in:

```go
func (ss *sqliteStorage) initialize() error {
	const version = 1
	steps := []dbmigration.Migration{
  	{
  		Up: func(tx *sql.Tx) error {
  			_, err := tx.Exec(`
  			CREATE TABLE IF NOT EXISTS metadata (
  				path TEXT PRIMARY KEY,
  				title TEXT,
  				created_at DATETIME,
  				last_edited DATETIME,
  				collection TEXT,
  				folders TEXT,
  				tags TEXT,
  				ancestor TEXT,
  				parents TEXT,
  				kids TEXT,
  				used_links TEXT,
  				links_to_here TEXT,
  				related TEXT,
  				editor TEXT,
  				size INTEGER,
  				"references" TEXT
  			);
  			CREATE INDEX IF NOT EXISTS idx_collection ON metadata(collection);
  			CREATE INDEX IF NOT EXISTS idx_editor ON metadata(editor);
  			`)
  			return err
  		},
  		Down: func(tx *sql.Tx) error {
  			_, err := tx.Exec(`DROP TABLE IF EXISTS metadata`)
  			return err
  		},
  	},
  	{Up: metaV1toV2, Down: metaV2toV1},
		{
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`UPDATE metadata SET test_col = 'migrated' WHERE test_col IS NULL`)
				return err
			},
			Down: func(tx *sql.Tx) error {
				_, err := tx.Exec(`UPDATE metadata SET test_col = NULL`)
				return err
			},
		},
		{
    Up: func(tx *sql.Tx) error {
        if _, err := tx.Exec(`ALTER TABLE metadata ADD COLUMN test_col TEXT`); err != nil {
            return err
        }
        return fmt.Errorf("intentional failure")
    },
    Down: nil,
},
	}
	if err := dbmigration.Migrate(ss.db, version, steps); err != nil {
		return fmt.Errorf("metadata storage migration failed: %w", err)
	}
	logging.LogDebug(logging.KeyApp, "metadata sqlite storage ready at version %d", version)
	return nil
}

func metaV1toV2(tx *sql.Tx) error {
	_, err := tx.Exec(`ALTER TABLE metadata ADD COLUMN test_col TEXT`)
	return err
}

func metaV2toV1(tx *sql.Tx) error {
	_, err := tx.Exec(`ALTER TABLE metadata DROP COLUMN test_col`)
	return err
}
```

**start application with different version**

```bash
# first check current version
sqlite3 storage/metadata/metadata.db "SELECT version FROM schema_version"
sqlite3 storage/metadata/metadata.db ".schema"

# set const version = 2
make dev   # stop immediately after "storage ready" log
sqlite3 storage/metadata/metadata.db "SELECT version FROM schema_version" # → 2
sqlite3 storage/metadata/metadata.db "PRAGMA table_info(metadata)" | grep test_col # → test_col should appear

# set const version = 1
make dev   # stop after startup
sqlite3 storage/metadata/metadata.db "SELECT version FROM schema_version" # → 1
sqlite3 storage/metadata/metadata.db "PRAGMA table_info(metadata)" | grep test_col # → nothing (column dropped)

# set const version = 3
# seed some rows first
sqlite3 storage/metadata/metadata.db "INSERT OR IGNORE INTO metadata (path) VALUES ('docs/test.md')"
make dev   # stop after startup
sqlite3 storage/metadata/metadata.db "SELECT path, test_col FROM metadata WHERE path = 'docs/test.md'" # → docs/test.md|migrated

# set const version = 4
make dev   # should log "migration 3→4 failed" and refuse to start
sqlite3 storage/metadata/metadata.db "SELECT version FROM schema_version" # → 3  (did not advance)
```

# Kanban

## Architecture

- Board is a **page shell + HTMX** pattern: `/kanban/{board}` renders the template, `GET /api/kanban/{board}` returns the column HTML on load and on filter change. `{board}` is a URL slug, not a raw folder path.
- Excerpts are built **inline** in `cardFromFile` (`kanban.ExcerptRunes` runes) and rendered with the card — one request per board instead of one per card
- Card moves are **optimistic UI** — the card is moved in the DOM immediately, then `POST /api/kanban/card/move` persists the tag change via `MetaDataMutate` (tags + kanban timestamps only; no derived-field recompute)

## Tag System

- Kanban state is stored as a regular metadata tag: `{prefix}-status-{status}` (e.g. `kb-status-inbox`)
- The prefix comes from `KNOV_KANBAN_PREFIX` (env, restart - changing it is a data-format change), valid statuses from the settings registry (`KanbanStatuses` in `settings_registry.go`)
- `sanitizeKanbanTags(oldTags, tags)` in `metadata.go` only checks *newly added* `{prefix}-*` tags (must be `{prefix}-status-{allowed status}`, a new status tag replaces the existing one) — tags already on the file are always kept, so a config change never deletes data (orphaned status tags just drop off the board). Called from `SetTags` / `PatchTags` / the metadata patch

## Boards

- Boards are explicitly configured folders, not auto-derived collections: the `KanbanBoards` setting (`folder/path:Display Name,...`), parsed per call by `configmanager.GetKanbanBoards()` into `[]KanbanBoard{FolderPath, DisplayName, Slug, FolderSync}`
- `Slug` is derived from `FolderPath` via `utils.GenerateID` (same helper used for markdown header/TOC anchor IDs) - duplicate slugs get a numeric suffix, not an error
- A file appears on a board if its directory equals the board's `FolderPath` or is nested under it (`kanban.folderMatches`, recursive match against `metadata.Folders` joined with `/`) - this is a superset of the old "same top-level collection" rule, not a replacement filter criterion in the generic filter engine
- `kanban.BuildBoard`/`TagsForFolder`/`FilesForFolder`/`GetOrder`/`SaveOrder` all take a literal folder path, not a slug - they have no dependency on `configmanager.GetKanbanBoards()` at all, which is what lets `kanbantest` seed/assert against a folder path directly with zero config plumbing
- `kanban.MoveCard(boardFolder, filePath, newStatus string)` takes the board folder explicitly so the event log entry is scoped to the board the move actually happened on - the drag-and-drop frontend already knows which board it's on (`window.KANBAN_CONFIG.board`), so `kanban.js` sends it as a `board` form field on every `/api/kanban/card/move` call, and `api_kanban.go` resolves it to a folder path before calling `MoveCard`. Boards are recursive (`projects/work` also covers `projects/work/urgent`), so guessing the board purely from the file's own directory is ambiguous whenever board folders overlap or nest - `kanban.resolveBoardFolder` (longest matching configured `FolderPath`) is kept only as a fallback for callers that don't know the board (empty `boardFolder`), not as the primary path

  *(This exact ambiguity caused a real regression during the folder-boards migration: with a board configured at `test` and every test suite's fixtures living under `docs/test/`, `resolveBoardFolder`'s longest-prefix guess collapsed `kanbantest`'s own fixture folder `test/kanban-tests` down to `test`, so its event-log assertion queried the wrong key and saw 0 events. Passing the board explicitly removes the guess entirely.)*
- The HTTP layer (`api_kanban.go`) is the only place that resolves slug → folder, via `configmanager.GetKanbanBoardBySlug`; unknown slugs 404
- `Metadata.Collection`/`CollectionFromPath` are untouched and still back browse-by-collection, the dashboard collections widget, the generic filter engine's `collection` criterion, etc. - kanban just no longer uses them for board scoping
- The recursive "is dirPath under folderPath" check is shared, not duplicated: `pathutils.FolderContains(dirPath, folderPath)` backs both `kanban.folderMatches`/`resolveBoardFolder` and `KNOV_AUTOCREATE_TAGS`'s folder scoping (`api_files.go`, via `files.FolderFromPath`) - same recursive semantics everywhere a folder scope is matched against a file's location
- `KNOV_AUTOCREATE_TAGS` (formerly two settings, `KNOV_AUTOCREATE_TAGS` + `KNOV_AUTOCREATE_COLLECTIONS`) is now a single list of `configmanager.AutoCreateTag{FolderPath, Tag}`: a bare entry (no `:`) means `FolderPath == ""`, applied to every new file; a `folder/path:tag` entry is folder-scoped and recursive, matching board-folder semantics instead of the old flat collection-equality check

## Key Files

| File                    | Role                                                                 |
| ----------------------- | -------------------------------------------------------------------- |
| `config.go`             | `GetKanbanPrefix/Statuses/Columns`, `GetKanbanBoards/GetKanbanBoardBySlug`, `IsKanbanTag`, `KanbanStatusTag` |
| `metadata.go`           | `sanitizeKanbanTags`, `SanitizeKanbanTags`                           |
| `kanban.go`             | `BuildBoard`, `TagsForFolder`, `FilesForFolder`, `MoveCard`, `folderMatches`, `resolveBoardFolder` |
| `api_kanban.go`         | board handler, move handler, excerpt handler, `resolveBoard` (slug→folder, 404 on unknown) |
| `render_kanban.go`      | `RenderKanbanCard`, `RenderKanbanColumn`                             |
| `static_kanban.css`     | all kanban styles (ID + class selectors)                             |
| `{theme}-kanban.gohtml` | page shell per theme                                                 |

## Config

- Only `KNOV_KANBAN_PREFIX` (tag format) and `KNOV_KANBAN_EVENTS_ENABLED` / `KNOV_KANBAN_EVENTS_STORAGE_PROVIDER` (storage init) are env vars; everything else is a runtime setting in the Kanban section of `settings_registry.go`

# Notifications

All toast notifications go through a single track:

1. Handler calls `notify.SetFlash(level, message)` — writes to cache storage under key `flash:notification`
2. After every htmx request and on page load, the JS in every page polls `GET /api/notifications/flash`
3. The endpoint calls `notify.ConsumeFlash()` — reads and deletes the entry, returns it as an `HX-Trigger` header
4. The existing `notify` JS event listener receives the trigger and renders the toast

This works for both in-page responses and cross-navigation responses (`HX-Redirect`, `HX-Refresh`) because the poll fires after the new page loads.

**Usage**

```go
import "knov/internal/server/notify"

// success
notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(lang, "file saved"))

// error (call before http.Error or writeResponse)
notify.SetFlash(notify.LevelError, translation.SprintfForRequest(lang, "failed to save"))
http.Error(w, "...", http.StatusInternalServerError)
```

# Async Jobs

`job.StartAsync(mu, job, args)` runs a job in the background and returns an id immediately, instead of blocking the request like `execute()` does.

- `internal/jobStorage` persists each run (id, type, args, status, timestamps, error) in sqlite, same shape/pattern as `notificationStorage`
- a job type opts into crash-recovery by implementing `Resumable() bool` and registering a reconstructor keyed by `Name()`; on startup `job.RecoverInterrupted()` re-runs everything still marked `running`, and marks non-resumable/unregistered types `interrupted` with a notification instead
- resumable jobs persist a resolved snapshot (e.g. a file list) rather than something re-derivable, so a resumed run can't pick up state changes made since the crash
- `GET /api/jobs/{id}` + `render.RenderJobStatus` return a self-polling htmx fragment that stops polling once the job reaches a terminal status

# Backup & Restore

`internal/backup` snapshots every StoragePath-backed storage (metadata, chat, kanban, notifications, config, search) into a single `.tar.gz` backup set. DataPath (docs/media) is already versioned by git, so it's out of scope. `cacheStorage` deliberately doesn't register - it holds nothing but data rebuilt from files/git on demand, so restoring an old cache snapshot would only reintroduce stale derived data; restore's recovery step refreshes it instead (`job.restoreJob`'s `afterRestore` flushes it via `files.CacheInvalidate` before the restart - a rebuild can't run pre-restart, restored storages' handles are already closed; `backuptest`'s `restoreAndReinit` rebuilds it in-process after reinit)

- storages self-register in their own `init()` via `backup.Register(name, ...)` — the same self-registration pattern `externalsuite.go` uses for test suites. `internal/backup` never imports a storage package directly, since every storage imports it for the `BackupSQLite`/`RestoreSQLite`/`BackupFile`/`RestoreFile` helpers, and an orchestrator-side import back would cycle
- sqlite storages snapshot via `VACUUM INTO` under their own existing read lock; json storages snapshot via a plain directory copy. `metadataStorage_yaml.go`'s front matter backend has no storage format of its own to copy (its data lives in docs files under DataPath, already versioned by git) — `Backup` instead snapshots front matter into a scratch sqlite db under the set, purely so a cross-backend restore has something to read (see `Migratable` below); `Restore` stays a no-op, since a same-backend restore should leave live docs alone rather than overwrite front matter git already tracks
- `backup.Run(target, names...)` backs up a subset of registered storages when given explicit names (e.g. just `"metadata"`), or everything when called with none — each set carries a `manifest.json` with the included storage names plus each one's `GetBackendType()` at backup time, so a partial set can be told apart from a full one later without extracting the rest, and so `Restore` can detect a storage whose backend has since changed; `backup.Manifest` reads just the storage-name list back out
- no cross-storage atomicity — storages are snapshotted sequentially, each under its own lock, not one transaction
- a partial backup failure aborts and discards the whole set rather than keeping an incomplete one
- restore takes a fresh **full** safety snapshot first (regardless of what the set being restored contains). Each sqlite storage's `Restore` reads and validates the backed-up database (`backup.ReadSQLiteBackup`) *before* closing its live db handle, so a corrupt or missing backup file fails without bricking a working connection — only once validated does it close the handle (Windows refuses to overwrite a file another handle still has open) and write the restored file (`backup.RestoreSQLite`). Restoring a partial set only touches the storages listed in the set's `manifest.json` (read back from the extracted archive), not "whichever subdirectories happen to exist" — a storage that was genuinely empty at backup time has no extracted subdirectory either, but is still restored (to empty) if the manifest lists it
- if a storage's manifest-recorded backend differs from the one currently configured (e.g. a sqlite-era backup restored after switching to json), applying it as-is would silently corrupt or no-op the live storage — `Restore` instead checks for `backup.Migratable` and calls `RestoreMigrate(srcDir, fromBackendType)`, which reads the backup with the old backend, wipes the live one (`Cleanup`), and writes the converted data into a freshly opened instance that's then reassigned as the live storage (mirroring `Init`'s own provider-migration path — see `kanbanStorage`/`metadataStorage`'s `restoreMigrate`). A storage that doesn't implement `Migratable` fails just that one storage's restore with an explicit error instead of silently applying mismatched data; an empty/missing recorded backend (backups made before this field existed) is treated as unknown and applied as-is via the ordinary same-backend path
- `backup.Restore(target, name, afterRestore func())` never hot-swaps any live `*sql.DB`/in-memory state itself — `afterRestore` is a required parameter, called synchronously exactly once whenever live storage may have been touched (full success, or the partial-failure `ErrRestoreIncomplete`) - deliberately conservative, since `Storage.Restore` reports no touched outcome of its own, so even a no-op restore (e.g. yaml metadata) can trigger it - so a future call site can't add itself while forgetting the recovery step. `job.restoreJob` uses it to restart the process for real; `backuptest`'s `restoreAndReinit` uses it to re-run every storage's `Init` in-process instead, since that suite never restarts
- `RestoreFile` clears the live directory before copying the backed-up files back in, so restore actually replaces state instead of overlaying onto it — a file created or renamed after the backup was taken doesn't survive a restore. Both `BackupFile` and `RestoreFile` write through `utils.WriteFileAtomic`, so an interrupted restore can't leave a live json/yaml file torn
- `manifest.json` is written as the first entry in the archive (not wherever the directory walk's alphabetical order happens to put it), so `backup.Manifest` only has to decompress the start of the archive, not scan past every storage's data first, to answer "what's in this set" for the `/system/backup` log page
- backup set names are timestamp-based at 1-second resolution; `Run` disambiguates with a `-N` suffix if that name is already taken on the target (e.g. a manual backup immediately followed by a restore's own pre-restore safety snapshot, landing in the same second) instead of one silently overwriting the other. A partial set's name gets a `_<storages>` suffix (e.g. `2026-08-08T21-10-29_metadata`); `backup.IsFullSet` checks for that underscore (the timestamp itself never contains one) to tell full and partial sets apart without opening the manifest
- `backup.Rotate(target, keepDays, keepFull)` — a set survives if it matches any of three independent rules: it's locked (`BackupTarget.Lock`/`Unlock`/`Locked`, a `<name>.locked` marker file next to the archive on `localTarget`, checked and toggled via `/system/backup`'s Lock/Unlock button), it's within `keepDays` of now (any kind, full or partial), or it's a full backup among the `keepFull` most recent full backups — a long-term floor so coming back after months away still leaves something restorable. Partial backups get no long-term floor of their own: once they age out of `keepDays` and aren't locked, they're deleted, so a partial set can never occupy the slot a full backup would otherwise have kept. `KNOV_BACKUP_ROTATION_KEEP_DAYS`/`KNOV_BACKUP_ROTATION_KEEP_FULL` configure the two counts; GFS-style date-bucketing was deliberately dropped in favor of this — not proportional to how little data/complexity budget a single-user app's backups need
- `BackupTarget` is byte-stream shaped (`io.Reader`/`io.ReadCloser`), so a remote target (S3, NFS) can be added later without touching `Run`/`Restore` — only a local-filesystem implementation (`job.DefaultBackupTarget`, rooted at `KNOV_BACKUPS_PATH`, default `./backups`) exists today
- `BackupTarget.LogEvent`/`Events` durably record every backup created and restore applied (`backup.Event{Kind, Set, Time}`, on `localTarget` a `log.json` array next to the archives, rewritten atomically) - kept independent of whether the referenced set still exists, so the history survives both rotation deleting the set later and a restore's own restart (which would otherwise wipe any purely in-memory record of the very event it just performed, unlike the in-memory `job.GetRecentRuns` history used by `/system/jobs`). `job.ListBackupLog` merges this with the target's current `List()`/`Locked()` state into `BackupLogEntry` (adding a synthetic "backup created" row, timestamped from the name, for any set that predates event logging or survived a lost log file) for the `/system/backup` page - a log, not just a listing: past events stay visible with a "no longer available" note and no actions once their set is gone, instead of just disappearing
- `job.OpenBackup` + `GET /api/system/backups/{name}/download` streams a set's raw archive back with `setAttachmentFilename` for the download link on each log row
- triggered manually from `/system/backup` (`job.RunBackup`/`RunRestore`, logged into `JobRun` history like `gitPushJob`), with a storage checkbox row for partial backups and a lock/unlock/download action per available log row
- optional scheduled backups: `KNOV_BACKUP_AUTO_ENABLED`/`KNOV_BACKUP_AUTO_CRON` (AppConfig env vars, restart required — same two-layer split as everything else in Configuration Management above) gate `job.checkAutoBackup`, ticked every `backupAutoCheckInterval` (fixed, 15m) from `job.Start()` — it's a no-op unless enabled and `backup.AutoBackupDue` finds the parsed cron schedule's (`backup.ParseCronSchedule`, `github.com/robfig/cron/v3`) next occurrence after the newest existing set (via `backup.ParseSetTime`) has already passed, so the check cadence and the actual backup cadence are decoupled. Driving this off the last set's own timestamp rather than a "ran today" flag means a device that isn't running 24/7 still catches up on a missed slot as soon as it's next on, instead of a slot that only ever lands outside its usage window getting skipped entirely

## Testing S3 backups with MinIO in Docker

### 1. Start MinIO

```bash
docker run -d --name knov-minio \
  -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=knovtest \
  -e MINIO_ROOT_PASSWORD=knovtest123 \
  minio/minio server /data --console-address ":9001"
```

- `:9000` = S3 API, `:9001` = web console (http://localhost:9001, login `knovtest` / `knovtest123`).

### 2. Create the bucket

Either click **Create Bucket → `knov-backups`** in the console, or one-shot with the mc client:

```bash
docker run --rm --network host --entrypoint sh minio/mc -c \
  "mc alias set local http://localhost:9000 knovtest knovtest123 && mc mb -p local/knov-backups"
```

### 3. Point the **dev** instance at it

Add to the dev `.env` (port 1324 — leave the real instance on 1325 alone):

```
KNOV_BACKUP_S3_BUCKET=knov-backups
KNOV_BACKUP_S3_ENDPOINT=localhost:9000
KNOV_BACKUP_S3_ACCESS_KEY=knovtest
KNOV_BACKUP_S3_SECRET_KEY=knovtest123
KNOV_BACKUP_S3_USE_SSL=false
KNOV_BACKUP_S3_REGION=us-east-1
KNOV_BACKUP_S3_PREFIX=dev/
```

`USE_SSL=false` because local MinIO is plain HTTP. `PREFIX` is optional.

### 4. Restart and exercise it

Restart the dev app, then at `/system/backup`:

1. **Create backup** → a `dev/<setname>.tar.gz` object should appear in the MinIO console.
2. **Lock** a set → a `dev/<setname>.locked` object appears; **Unlock** removes it.
3. **Download** → streams the archive straight from MinIO.
4. **Restore** → takes a fresh safety snapshot (new object), then restarts.
5. Create several backups and confirm rotation deletes the oldest objects per `KNOV_BACKUP_ROTATION_KEEP_DAYS` / `_KEEP_DEFAULT`.
6. Check `dev/log.json` in the bucket — the append-only backup/restore history.

### 5. Cleanup

```bash
docker rm -f knov-minio
```

Then remove the `KNOV_BACKUP_S3_*` lines from `.env` to fall back to local `KNOV_BACKUPS_PATH`.

# Editor Types

Each file can have an editor type stored in its metadata (`editor` field). The type controls which editor opens when the file is edited. The editor is resolved in this order: explicit metadata → file extension → parser detection → default (toastui).

| Editor type | Value | Description |
|---|---|---|
| ToastUI (default) | `toastui-editor` | Markdown WYSIWYG editor with toolbar, preview, media upload, wiki-link autocomplete |
| CodeMirror | `codemirror-editor` | Plain text editor, no toolbar, vim keybindings enabled by default — distraction-free writing |
| Textarea | `textarea-editor` | Raw textarea, minimal, used for non-markdown files (e.g. DokuWiki) |
| List | `list-editor` | Drag-and-drop ordered list editor, saves as markdown |
| Todo | `todo-editor` | Checkbox task list editor using GFM `- [ ]` syntax |
| Filter | `filter-editor` | Visual query builder for filter files (`.filter`) |
| Index / MOC | `index-editor` | Ordered link list editor for index/map-of-content files (`.index`, `.moc`) |
| Book | `book-editor` | Index editor in "book" mode (`.book`): entries carry an optional `#section` and subheaders flag, and the file view/export shows the composed document (referenced files and sections inlined into one markdown doc) rather than the raw entry list |

Or via file extension — certain extensions map automatically: `.filter` → filter-editor, `.list` → list-editor, `.todo` → todo-editor, `.index` / `.moc` → index-editor, `.book` → book-editor, `.txt` → textarea-editor.

## build the codemirror editor

https://github.com/blueberrycongee/codemirror-live-markdown

```bash
mkdir ~/codemirror-bundle && cd ~/codemirror-bundle
npm init -y
npm install @codemirror/state @codemirror/view @codemirror/commands @codemirror/search @codemirror/language @codemirror/lang-markdown @codemirror/autocomplete @codemirror/lint @replit/codemirror-vim @uiw/codemirror-extensions-line-numbers-relative
npm install --save-dev esbuild

touch editor.js
touch build.sh
chmod +x build.sh
```

editor.js
```js
import { EditorState, EditorSelection, RangeSetBuilder } from "@codemirror/state";
import {
  EditorView,
  keymap,
  gutter,
  GutterMarker,
  drawSelection,
  highlightActiveLine,
  placeholder,
  lineNumbers,
  highlightSpecialChars,
  Decoration,
  ViewPlugin,
} from "@codemirror/view";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import {
  search,
  searchKeymap,
  highlightSelectionMatches,
} from "@codemirror/search";
import { vim, Vim, getCM } from "@replit/codemirror-vim";
import {
  bracketMatching,
  foldGutter,
  syntaxHighlighting,
  HighlightStyle,
  foldKeymap,
  indentUnit,
  syntaxTree,
} from "@codemirror/language";
import { classHighlighter, tags } from "@lezer/highlight";
import { markdown } from "@codemirror/lang-markdown";
import { javascript } from "@codemirror/lang-javascript";
import { python } from "@codemirror/lang-python";
import { css } from "@codemirror/lang-css";
import { html } from "@codemirror/lang-html";
import { json } from "@codemirror/lang-json";
import { sql } from "@codemirror/lang-sql";
import { yaml } from "@codemirror/lang-yaml";
import { go } from "@codemirror/lang-go";
import { StreamLanguage } from "@codemirror/language";
import { shell } from "@codemirror/legacy-modes/mode/shell";
import {
  autocompletion,
  completionKeymap,
  closeBrackets,
  closeBracketsKeymap,
} from "@codemirror/autocomplete";

// ── Fenced code block languages ─────────────────────────────────────────────
//
// Gives ```js / ```python / … fenced blocks a real language parser, so their
// content gets live syntax highlighting (keywords, strings, …) instead of
// being treated as plain monospace text. classHighlighter already maps the
// resulting tags (tags.keyword, tags.string, tags.typeName, …) to tok-*
// classes, which index.html already styles.
const shellLanguage = StreamLanguage.define(shell);
const codeLanguageByAlias = {
  js: javascript(),
  jsx: javascript({ jsx: true }),
  javascript: javascript(),
  mjs: javascript(),
  cjs: javascript(),
  ts: javascript({ typescript: true }),
  tsx: javascript({ typescript: true, jsx: true }),
  typescript: javascript({ typescript: true }),
  py: python(),
  python: python(),
  css: css(),
  html: html(),
  htm: html(),
  json: json(),
  sql: sql(),
  yaml: yaml(),
  yml: yaml(),
  go: go(),
  golang: go(),
  sh: shellLanguage,
  bash: shellLanguage,
  shell: shellLanguage,
};

function codeLanguages(info) {
  const name = info.trim().split(/\s+/)[0].toLowerCase();
  const support = codeLanguageByAlias[name];
  // LanguageSupport instances (javascript(), python(), …) need unwrapping to
  // their .language; StreamLanguage instances (shellLanguage) already are one.
  return support ? support.language || support : null;
}

// ── Live-preview markdown highlighting ──────────────────────────────────────
//
// Goal: content renders styled (real heading sizes, real bold/italic, code
// pills, …) while the markdown syntax markers (#, **, _, `, >, -, [...]())
// stay visible at all times — no widget replacement / hide-on-blur tricks,
// just CSS driven by the syntax tree. `classHighlighter` supplies generic
// classes (tok-strong, tok-link, tok-comment, …); this style adds the
// markdown-specific ones classHighlighter doesn't cover (per-level headings,
// inline code, quotes, lists, hr, and the dimmed "marker" look).
const markdownLiveStyle = HighlightStyle.define([
  { tag: tags.heading1, class: "cm-h1" },
  { tag: tags.heading2, class: "cm-h2" },
  { tag: tags.heading3, class: "cm-h3" },
  { tag: tags.heading4, class: "cm-h4" },
  { tag: tags.heading5, class: "cm-h5" },
  { tag: tags.heading6, class: "cm-h6" },
  { tag: tags.strikethrough, class: "cm-strike" },
  { tag: tags.monospace, class: "cm-inline-code" },
  { tag: tags.quote, class: "cm-quote" },
  { tag: tags.list, class: "cm-list" },
  { tag: tags.contentSeparator, class: "cm-hr" },
  { tag: tags.processingInstruction, class: "cm-mark" },
]);

// ── WYSIWYG marker hiding ───────────────────────────────────────────────────
//
// Conceals markdown syntax markers (#, **, `, >, -, [...]()) everywhere except
// on the line the selection currently touches — same idea as Obsidian/Typora
// "live preview". Implemented as replace decorations (zero-width, so the
// marker text is gone from layout) rather than a CSS hide, and registered as
// EditorView.atomicRanges so arrow-key/mouse navigation steps over a hidden
// marker as a single unit instead of getting stuck inside it.
const WYSIWYG_HIDDEN_TYPES = new Set([
  "HeaderMark",
  "QuoteMark",
  "ListMark",
  "LinkMark",
  "EmphasisMark",
  "CodeMark",
  "CodeInfo",
  "StrikethroughMark",
  "URL",
  "LinkTitle",
]);
// These mark types are followed by a literal space that isn't part of any
// node ("# heading", "> quote", "- item") — swallow it too, or hiding the
// mark alone leaves a stray leading space.
const WYSIWYG_TRIM_TRAILING_SPACE = new Set(["HeaderMark", "QuoteMark", "ListMark"]);

const hiddenMarkDecoration = Decoration.replace({});

function wysiwygMarkRange(doc, node) {
  let to = node.to;
  if (WYSIWYG_TRIM_TRAILING_SPACE.has(node.name) && doc.sliceString(to, to + 1) === " ") {
    to += 1;
  }
  return { from: node.from, to };
}

function wysiwygLineTouchesSelection(state, from, to) {
  const startLine = state.doc.lineAt(from).number;
  const endLine = state.doc.lineAt(Math.max(from, to - 1)).number;
  return state.selection.ranges.some((range) => {
    const anchorLine = state.doc.lineAt(range.anchor).number;
    const headLine = state.doc.lineAt(range.head).number;
    return (
      (anchorLine >= startLine && anchorLine <= endLine) ||
      (headLine >= startLine && headLine <= endLine)
    );
  });
}

function computeWysiwygDecorations(view) {
  const builder = new RangeSetBuilder();
  const { state } = view;
  for (const { from, to } of view.visibleRanges) {
    syntaxTree(state).iterate({
      from,
      to,
      enter: (node) => {
        if (!WYSIWYG_HIDDEN_TYPES.has(node.name)) return;
        const range = wysiwygMarkRange(state.doc, node);
        if (wysiwygLineTouchesSelection(state, range.from, range.to)) return;
        builder.add(range.from, range.to, hiddenMarkDecoration);
      },
    });
  }
  return builder.finish();
}

const wysiwygMarkerHiding = ViewPlugin.fromClass(
  class {
    constructor(view) {
      this.decorations = computeWysiwygDecorations(view);
    }
    update(update) {
      if (update.docChanged || update.selectionSet || update.viewportChanged) {
        this.decorations = computeWysiwygDecorations(update.view);
      }
    }
  },
  {
    decorations: (plugin) => plugin.decorations,
    provide: (pluginClass) =>
      EditorView.atomicRanges.of(
        (view) => view.plugin(pluginClass)?.decorations || Decoration.none
      ),
  }
);

// Relative line numbers gutter: "0" on the cursor line, absolute distance on others.
function makeRelativeLineNumbers() {
  return gutter({
    class: "cm-lineNumbers",
    lineMarkerChange: (update) => update.selectionSet || update.docChanged,
    lineMarker: (view, line) => {
      const curLine = view.state.doc.lineAt(
        view.state.selection.main.head
      ).number;
      const thisLine = view.state.doc.lineAt(line.from).number;
      const label =
        thisLine === curLine ? "0" : String(Math.abs(thisLine - curLine));
      return Object.assign(Object.create(GutterMarker.prototype), {
        toDOM() {
          const d = document.createElement("div");
          d.textContent = label;
          return d;
        },
        eq(other) {
          return other.toDOM && other.toDOM().textContent === label;
        },
      });
    },
  });
}

// ── Vim clipboard integration ─────────────────────────────────────────────────
//
// WRITE (y/d/c → system clipboard):
//   Patch RegisterController.prototype.pushText so every unnamed-register write
//   also calls navigator.clipboard.writeText().  Prototype patch survives
//   resetVimGlobalState_() calls that replace the controller instance.
//
// READ (system clipboard → p):
//   codemirror-vim already supports the "+" register: its paste action calls
//   navigator.clipboard.readText() when registerName === '+'.  We use
//   Vim.noremap to redirect p/P → "+p/"+P so pressing p always reads the
//   system clipboard via vim's own built-in path — no custom keydown intercept.
//
//   The first time p is pressed Firefox will show a one-time "Paste" permission
//   prompt.  After the user allows it once the permission is permanent.
//   To make the prompt appear on editor FOCUS (more natural than on p), we call
//   readText() in a focus handler so the permission is already granted by the
//   time the user reaches for p.

let _protoHooked = false;

function _hookVimPrototype() {
  if (_protoHooked || typeof navigator === "undefined" || !navigator.clipboard)
    return;
  try {
    const rc = Vim.getRegisterController();
    if (!rc) return;
    const proto = Object.getPrototypeOf(rc);
    if (proto._clipboardPatched) return;
    proto._clipboardPatched = true;
    _protoHooked = true;

    const origPushText = proto.pushText;
    proto.pushText = function (registerName, op, text, linewise, blockwise) {
      origPushText.call(this, registerName, op, text, linewise, blockwise);
      if (!registerName || registerName === '"') {
        const stored = this.unnamedRegister && this.unnamedRegister.toString();
        if (stored) navigator.clipboard.writeText(stored).catch(() => {});
      }
    };
  } catch (_) {}
}

// On editor focus: call readText() so the browser permission prompt (if any)
// appears when the user clicks into the editor, not when they press p.
// After the one-time grant this is a silent background sync.
function vimFocusClipboardSync() {
  let denied = false;
  return EditorView.domEventHandlers({
    focus() {
      if (!navigator.clipboard || denied) return;
      navigator.clipboard.readText().then(
        (text) => {
          if (!text) return;
          try {
            const rc = Vim.getRegisterController();
            if (rc) rc.unnamedRegister.setText(text, text.endsWith("\n"));
          } catch (_) {}
        },
        (e) => { if (e && e.name === "NotAllowedError") denied = true; }
      );
    },
  });
}

// ─────────────────────────────────────────────────────────────────────────────

window.createCodeMirror = function (element, content, options) {
  options = options || {};

  const relLineNums = makeRelativeLineNumbers();

  const extensions = [
    ...(options.vimMode ? [vim()] : []),

    history(),
    drawSelection(),
    EditorView.lineWrapping,

    markdown({ codeLanguages }),
    syntaxHighlighting(classHighlighter),
    syntaxHighlighting(markdownLiveStyle),
    ...(options.wysiwyg ? [wysiwygMarkerHiding] : []),

    ...(options.bracketMatching ? [bracketMatching()] : []),
    ...(options.autoBrackets ? [closeBrackets()] : []),
    autocompletion(),
    indentUnit.of("  "),

    highlightActiveLine(),
    ...(options.lineNumbers && !options.relativeLineNumbers
      ? [lineNumbers()]
      : []),
    ...(options.relativeLineNumbers ? [relLineNums] : []),
    ...(options.highlightSelection
      ? [
          highlightSelectionMatches(
            options.highlightSelectionWholeWord ? { wholeWords: true } : {}
          ),
        ]
      : []),
    highlightSpecialChars(),
    ...(options.foldGutter ? [foldGutter()] : []),

    search(),

    keymap.of([
      ...defaultKeymap,
      ...historyKeymap,
      ...searchKeymap,
      ...completionKeymap,
      ...(options.autoBrackets ? closeBracketsKeymap : []),
      ...(options.foldGutter ? foldKeymap : []),
    ]),

    EditorView.inputHandler.of(typeToWrap),

    EditorView.updateListener.of(function (update) {
      if (options.onChange) options.onChange(update);
    }),

    ...(options.vimMode ? [vimFocusClipboardSync()] : []),
  ];

  if (options.placeholder) {
    extensions.push(placeholder(options.placeholder));
  }

  const state = EditorState.create({ doc: content || "", extensions });
  const view = new EditorView({ state, parent: element });

  if (options.vimMode) {
    _hookVimPrototype();
    // Redirect p/P to the "+" register so vim's built-in readText() path is used.
    Vim.noremap("p", '"+p', "normal");
    Vim.noremap("P", '"+P', "normal");
  }

  return view;
};

// ── Type-to-wrap ─────────────────────────────────────────────────────────────
//
// Typing one of these characters over a non-empty selection wraps the
// selection instead of replacing it (default contenteditable behavior is to
// delete the selection and insert the typed character). The wrapped text
// stays selected, so pressing the same character again stacks another layer
// around it — e.g. selecting "word" and pressing * twice gives **word**.
//
// Deliberately not the toolbar's wrapSelection(): that toggles markers off
// when the selection is already surrounded by them, which would make a
// second keypress of the same character undo the first instead of stacking.
const TYPE_TO_WRAP_CHARS = new Set(["`", "*", "_", "~"]);

function insertWrap(view, char) {
  view.dispatch(
    view.state.changeByRange((range) => ({
      changes: [{ from: range.from, insert: char }, { from: range.to, insert: char }],
      range: EditorSelection.range(range.from + char.length, range.to + char.length),
    }))
  );
}

function typeToWrap(view, from, to, text) {
  if (text.length !== 1 || !TYPE_TO_WRAP_CHARS.has(text)) return false;
  if (view.state.selection.ranges.every((range) => range.empty)) return false;
  insertWrap(view, text);
  return true;
}

// ── Toolbar commands ──────────────────────────────────────────────────────
//
// Small, dependency-free markdown edit commands used by the toolbar buttons
// in index.html. Each takes the live EditorView and mutates it directly.

// Wrap/unwrap every selection range with `before`...`after` (e.g. "**" for
// bold). Toggles off when the selection (or its immediate surroundings) is
// already wrapped; on an empty selection it drops the cursor between the
// inserted markers so typing continues immediately.
function wrapSelection(view, before, after) {
  after = after == null ? before : after;
  const doc = view.state.doc;
  view.dispatch(
    view.state.changeByRange((range) => {
      const { from, to } = range;
      const selText = doc.sliceString(from, to);

      if (
        selText.length >= before.length + after.length &&
        selText.startsWith(before) &&
        selText.endsWith(after)
      ) {
        const inner = selText.slice(before.length, selText.length - after.length);
        return {
          changes: [{ from, to, insert: inner }],
          range: EditorSelection.range(from, from + inner.length),
        };
      }

      const outerBefore = doc.sliceString(Math.max(0, from - before.length), from);
      const outerAfter = doc.sliceString(to, Math.min(doc.length, to + after.length));
      if (outerBefore === before && outerAfter === after) {
        return {
          changes: [
            { from: from - before.length, to: from, insert: "" },
            { from: to, to: to + after.length, insert: "" },
          ],
          range: EditorSelection.range(from - before.length, to - before.length),
        };
      }

      return {
        changes: [{ from, insert: before }, { from: to, insert: after }],
        range: range.empty
          ? EditorSelection.cursor(from + before.length)
          : EditorSelection.range(from + before.length, to + before.length),
      };
    })
  );
  view.focus();
}

// Toggle a per-line prefix (list bullet, quote, …) across every line the
// current selection touches. Removes the prefix if every touched line
// already has it, otherwise adds it to whichever lines are missing it.
function toggleLinePrefix(view, regex, prefix) {
  const state = view.state;
  const range = state.selection.main;
  const startLine = state.doc.lineAt(range.from).number;
  const endLine = state.doc.lineAt(range.to).number;

  let allHave = true;
  for (let n = startLine; n <= endLine; n++) {
    if (!regex.test(state.doc.line(n).text)) {
      allHave = false;
      break;
    }
  }

  const changes = [];
  for (let n = startLine; n <= endLine; n++) {
    const line = state.doc.line(n);
    if (allHave) {
      const m = regex.exec(line.text);
      changes.push({ from: line.from, to: line.from + m[0].length, insert: "" });
    } else if (!regex.test(line.text)) {
      changes.push({ from: line.from, insert: prefix });
    }
  }
  if (!changes.length) return;

  const changeSet = state.changes(changes);
  view.dispatch({
    changes,
    selection: EditorSelection.range(
      changeSet.mapPos(range.anchor),
      changeSet.mapPos(range.head)
    ),
    scrollIntoView: true,
  });
  view.focus();
}

// Toggle an ATX heading marker ("#".repeat(level) + " ") on the current line.
// Re-clicking the same level removes it; clicking a different level swaps it.
function toggleHeading(view, level) {
  const marker = "#".repeat(level) + " ";
  view.dispatch(
    view.state.changeByRange((range) => {
      const line = view.state.doc.lineAt(range.from);
      const match = /^(#{1,6})\s+/.exec(line.text);
      let changes, delta;
      if (match && match[1].length === level) {
        changes = { from: line.from, to: line.from + match[0].length, insert: "" };
        delta = -match[0].length;
      } else if (match) {
        changes = { from: line.from, to: line.from + match[0].length, insert: marker };
        delta = marker.length - match[0].length;
      } else {
        changes = { from: line.from, insert: marker };
        delta = marker.length;
      }
      const pos = Math.max(line.from, range.head + delta);
      return { changes, range: EditorSelection.cursor(pos) };
    })
  );
  view.focus();
}

function insertCodeBlock(view) {
  const range = view.state.selection.main;
  const selText = view.state.sliceDoc(range.from, range.to);
  const before = "```\n";
  const after = "\n```";
  view.dispatch({
    changes: { from: range.from, to: range.to, insert: before + selText + after },
    selection: EditorSelection.range(
      range.from + before.length,
      range.from + before.length + selText.length
    ),
  });
  view.focus();
}

function insertLink(view) {
  const range = view.state.selection.main;
  const selText = view.state.sliceDoc(range.from, range.to);
  const text = selText || "link text";
  const url = "https://";
  const insert = `[${text}](/files/$%7Burl%7D.md)`;
  const urlStart = range.from + 1 + text.length + 2;
  view.dispatch({
    changes: { from: range.from, to: range.to, insert },
    selection: EditorSelection.range(urlStart, urlStart + url.length),
  });
  view.focus();
}

function insertHr(view) {
  const state = view.state;
  const pos = state.selection.main.to;
  const line = state.doc.lineAt(pos);
  const insert = (line.text.length ? "\n\n" : "\n") + "---\n\n";
  view.dispatch({
    changes: { from: pos, insert },
    selection: EditorSelection.cursor(pos + insert.length),
  });
  view.focus();
}

function insertTable(view) {
  const state = view.state;
  const pos = state.selection.main.to;
  const line = state.doc.lineAt(pos);
  const table = "| Header | Header |\n| --- | --- |\n| Cell | Cell |\n";
  const insert = (line.text.length ? "\n\n" : "\n") + table + "\n";
  view.dispatch({
    changes: { from: pos, insert },
    selection: EditorSelection.cursor(pos + insert.length),
  });
  view.focus();
}

window.mdCommands = {
  bold: (view) => wrapSelection(view, "**"),
  italic: (view) => wrapSelection(view, "_"),
  strikethrough: (view) => wrapSelection(view, "~~"),
  inlineCode: (view) => wrapSelection(view, "`"),
  heading: (view, level) => toggleHeading(view, level),
  quote: (view) => toggleLinePrefix(view, /^>\s?/, "> "),
  bulletList: (view) => toggleLinePrefix(view, /^[-*]\s/, "- "),
  orderedList: (view) => toggleLinePrefix(view, /^\d+\.\s/, "1. "),
  codeBlock: (view) => insertCodeBlock(view),
  link: (view) => insertLink(view),
  hr: (view) => insertHr(view),
  table: (view) => insertTable(view),
};
```

build.sh
```bash
#!/bin/bash

OUTPUTFILE=output/codemirror6-bundle.min.js
npx esbuild editor.js --bundle --minify --outfile=$OUTPUTFILE
ls -lh $OUTPUTFILE
```

## Editors considered and rejected

- **overtype** (https://github.com/panphora/overtype) — worked fine, delivered the same result as the custom codemirror6 build; more lightweight and might handle larger files better
- **toastui** (https://ui.toast.com/tui-editor) — annoying to customize, small UI problems (copy/paste, selection text)
- **quikdown** (https://github.com/deftio/quikdown) — ok, not tested for long
- Not evaluated: editor.md (https://github.com/pandao/editor.md), traven (https://github.com/slpstream/traven), codemirror5 (https://github.com/codemirror/codemirror5), codejar (https://medv.io/codejar/)

# Creating a Custom Theme

See [`docs/create_your_own_theme.md`](create_your_own_theme.md).

# System Pages

`/system/*` (`changelog`, `logs`, `version`, `jobs`, `backup`, `environment`) is a namespace for app-internal pages whose **content is controlled by the application**, not theme templates.

**How it works**

- `ThemeManager.RenderSystemPage(w, title, content)` renders a page using the current theme's `base.gohtml`
- the content block is an inline constant in `internal/thememanager/system.go`
- nothing theme authors can override

## /system/changelog

Renders all changelog markdown files from `docs/changelogs/` merged and sorted newest-first, displayed in the standard file view layout.

## /system/logs

Live in-app log viewer. Polls the in-memory ring buffer every 5 seconds. Supports client-side text filter, pause toggle, and — when file logging is active — a full-file view and download link.

## /system/version

Shows `.Version` and `.BuildTime` (see [Versioning](#versioning) below).

## /system/jobs

Table of recent background job runs (name, start/finish time, duration, status, error), polling `/api/system/jobs` every 3 seconds.

## /system/backup

A log of every backup created and restore applied, newest first, plus a "create backup" action and per-row restore/lock/download actions (hidden once a row's set is no longer available). See [Backup & Restore](#backup--restore) below.

## /system/environment

Every documented `KNOV_*` env var (`configmanager.EnvVarDefs`), grouped by category, with description, options, current value and default. Sensitive values (git password/token) are redacted. `internal/server/render/render_system.go`'s `RenderEnvironmentTable`/`RenderEnvironmentSummary` render the full table and the compact admin-panel summary respectively, both fed by `/api/system/environment` (`?view=summary` for the compact form) so the page and the admin panel never drift apart.

## Adding a new system page

- Add a handler in `internal/server/server.go`
- Register a route under `/system/`
- Call `tm.RenderSystemPage(w, title, content)` — no template file needed

# Filter

I added filter tests for these cases:

- each field at least once:
  - title
  - collection
  - tags
  - editor
  - createdAt
  - lastEdited
  - folders
  - child of
  - parent of
  - ancestor of
  - references
- at least 2 tests with both and/or
- at least 2 tests with both include/exclude
- each operator at least once:
  - equals
  - contains
  - regex
  - greater than
  - less than
  - in array
- at least 2 with multiple filter
- date equals
- date contains
- date regex

# Versioning

Version format: `<year>-<commitcount>-<hash>` — e.g. `2026-142-a3f4b2`.

No version file to maintain. Everything is derived from git at build time (prod) or startup (dev).

**prod** (`make prod`) — injected via `-ldflags`, burned into the binary:

```makefile
VERSION    := $(shell date -u '+%Y')-$(shell git rev-list --count HEAD)-$(shell git rev-parse --short HEAD)
BUILD_TIME := $(shell date -u '+%Y-%m-%d %H:%M')
LDFLAGS    := -ldflags "-X 'knov/internal/version.Version=$(VERSION)' \
                         -X 'knov/internal/version.BuildTime=$(BUILD_TIME) UTC'"
```

**dev** (`make dev`, `go run`) — computed at startup in `internal/version/version.go` via `init()`, appends `-dev`:

```
2026-142-a3f4b2-dev
```

| Situation | Version | Build time |
|---|---|---|
| `make prod` | `2026-142-a3f4b2` | build time |
| `make dev` / `go run` | `2026-142-a3f4b2-dev` | startup time |

Both values are available at `/system/version` and in every template via `.Version` and `.BuildTime`.

**Creating a release**

Tag the commit you want to release, then create a release on Codeberg/GitHub from that tag:

```bash
git tag 2026-142-a3f4b2   # use the version string shown on /system/version
git push origin 2026-142-a3f4b2
```

# Changelog

Changelogs are auto-generated from git commit history using [Conventional Commits](https://www.conventionalcommits.org/).

Generated files live in `docs/changelogs/<year>.md` (e.g. `docs/changelogs/2026.md`), one file per year, months grouped newest-first. The `docs/` folder is embedded at build time so the `/changelog` route can serve them without any external files.

**Commit Types**

| Type                                                                 | Section          | Description         |
| -------------------------------------------------------------------- | ---------------- | ------------------- |
| `feat:`                                                              | features         | new feature         |
| `fix:`                                                               | fixes            | bug fix             |
| `build:` `chore:` `ci:` `docs:` `style:` `refactor:` `perf:` `test:` | other            | everything else     |
| `feat!:` / `BREAKING CHANGE:` footer                                 | breaking changes | breaking API change |

Scopes are supported: `feat(kanban): add drag drop` is treated the same as `feat: add drag drop`.

Commits that don't match any type prefix are silently ignored.

**Generation**

```bash
# full rebuild from entire git history
make changelog
```

`make dev` and `make prod` both run `make changelog` automatically before building, so `docs/changelogs/` is always up to date in the embedded binary.
