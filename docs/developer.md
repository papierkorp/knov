# Developer Guide

**Prerequisites**

- Go 1.21 or later
- Git
- Make
- Swag CLI: `go install github.com/swaggo/swag/cmd/swag@latest`
- gotext: `go install golang.org/x/text/cmd/gotext@latest`

## Quick Start

Clone and setup:

```bash
git clone https://github.com/papierkorp/knov.git
cd knov
go mod download
```

Install required tools:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
go install golang.org/x/text/cmd/gotext@latest
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

# Create and Run Docker image
make docker
```

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

Generate translations:

```bash
make translation
```

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

## Filter File - System

**Storage**

- Filter configs are stored in configStorage (JSON) under the key `filter/<filterID>`
- The filter ID is a unique path-like string, e.g. `my/notes-filter`

**Paired Index File**

- Every filter has a paired physical index file in `data/docs/`
- Path: `<filterID>` + extension from `KNOV_USE_EXTENSION_INDEX` (`.index` or `.md`)
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
	logging.LogDebug("metadata sqlite storage ready at version %d", version)
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

- Board is a **page shell + HTMX** pattern: `/kanban/{collection}` renders the template, `GET /api/kanban/{collection}` returns the column HTML on load and on filter change
- Excerpts are **lazy-loaded** per card via `GET /api/kanban/excerpt?filepath=...&chars=30`
- Card moves are **optimistic UI** — the card is moved in the DOM immediately, then `POST /api/kanban/card/move` persists the tag change using `MetaDataSaveRaw` (skips parent/link processing)

## Tag System

- Kanban state is stored as a regular metadata tag: `{prefix}-status-{status}` (e.g. `kb-status-inbox`)
- Prefix and valid statuses come from env: `KNOV_KANBAN_PREFIX`, `KNOV_KANBAN_STATUS`
- `sanitizeKanbanTags()` in `metadata.go` enforces: one kanban tag max, known sub-namespace only (`status` for now), status must be in allowlist — called on every `MetaDataSave`
- Adding a new sub-namespace (e.g. `kb-priority-*`): add it to `knownSubNamespaces` in `sanitizeKanbanTags`

## Collection

- A file appears on a board only if `metadata.Collection == {collection}` — collection is derived from the file's first-level folder automatically
- Root-level files (`collection: ""`) never appear on any board

## Key Files

| File                    | Role                                                                 |
| ----------------------- | -------------------------------------------------------------------- |
| `config.go`             | `GetKanbanPrefix/Statuses/Columns`, `IsKanbanTag`, `KanbanStatusTag` |
| `metadata.go`           | `sanitizeKanbanTags`, `SanitizeKanbanTags`                           |
| `api_kanban.go`         | board handler, move handler, excerpt handler, `extractExcerpt`       |
| `render_kanban.go`      | `RenderKanbanCard`, `RenderKanbanColumn`                             |
| `static_kanban.css`     | all kanban styles (ID + class selectors)                             |
| `{theme}-kanban.gohtml` | page shell per theme                                                 |

## Env Vars

```
KNOV_KANBAN_PREFIX=kb          # tag prefix
KNOV_KANBAN_STATUS=inbox,inprogress,blocked,archive  # all valid statuses
KNOV_KANBAN_COLUMNS=inbox,inprogress,blocked         # visible columns (subset)
```

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

# Creating a Custom Theme

## Structure

A theme lives in `themes/<your-theme-name>/` and requires:

- `theme.json` — metadata and settings schema
- `style.css` — main stylesheet, loaded via `/themes/<name>/style.css`
- One `.gohtml` file per template (see `ThemeTemplates` in `internal/thememanager/thememanager.go` for all required names)

## theme.json

Defined by `ThemeMetadata` in `internal/thememanager/thememanager.go`. Required fields: `name`, `version`, `author`, `description`. Optional: `themeSettings` — a map of setting keys to `ThemeSetting` objects (type, default, label). Settings are exposed in the UI automatically and available in templates via `.ThemeSettings`.

Setting types supported: `boolean`, `select` (with `options`), `textarea`, `number` (with optional `min`/`max`).

## Templates

- Every template must define a `content` block consumed by `base.gohtml`
- `base.gohtml` is the layout shell — it must include the theme stylesheet, HTMX, and any other global scripts your theme needs
- Static editor assets (markdown editor, list editor, filter editor CSS/JS, etc.) are **injected automatically** before `</head>` and `</body>` — do not add them manually
- Template data structs are defined in `internal/server/thememanager/template_data.go` — reference these to know what fields are available per page
- Translation is available via the `T` template function

## CSS Conventions

- Use CSS custom properties (`--primary`, `--bg`, `--text`, `--border`, etc.) for colors so the theme responds to dark mode and color scheme settings
- Global styles go in `style.css`; page/component-specific styles should use ID selectors (`#page-*`, `#component-*`, `#view-*`) to avoid conflicts with injected editor styles
- Dark mode is driven by `data-dark-mode="true"` on `<body>`; color scheme by `data-color-scheme`

## Loading

- Themes are loaded from `themes/` at startup by `internal/thememanager/thememanager.go` => `loadAllThemes()`
- The folder name becomes the theme identifier used in settings
- `themes/overwrite/` is reserved and skipped during theme discovery
- Switch themes via **Settings => Theme** — no restart required

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

Version and build time are injected at link time via `-ldflags` — no file is written, the values are burned directly into the binary.

The source of truth is `internal/version/version.go`:

```go
var Version   = "dev"
var BuildTime = "unknown"
```

These defaults are used by `make dev` (`go run`, no ldflags). `make prod` overwrites them:

```makefile
VERSION    := $(shell git describe --tags --always --dirty)
BUILD_TIME := $(shell date -u '+%Y-%m-%d %H:%M')
LDFLAGS    := -ldflags "-X 'knov/internal/version.Version=$(VERSION)' \
                         -X 'knov/internal/version.BuildTime=$(BUILD_TIME) UTC'"
```

**Output depending on context**

| Situation | Example value |
|---|---|
| `make dev` | `dev` / `unknown` |
| `make prod`, no tags yet | `abc1234` / `2026-06-11 14:32 UTC` |
| `make prod`, after `git tag v0.2.0` | `v0.2.0` / `2026-06-11 14:32 UTC` |
| `make prod`, commits after last tag | `v0.2.0-3-gabc1234` / `2026-06-11 14:32 UTC` |
| `make prod`, uncommitted changes | `v0.2.0-dirty` / `2026-06-11 14:32 UTC` |

Both values are available in every template via `.Version` and `.BuildTime` (added to `BaseTemplateData`). They are displayed at the top of the Environment Info section on the settings page.

**Creating a release**

```bash
git tag v0.2.0
git push --tags
# CI runs make prod → builds knov-v0.2.0-linux and knov-v0.2.0-windows.exe
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
