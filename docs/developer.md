# Developer Guide

**Prerequisites**

- Go 1.21 or later
- Git
- Make
- Swag CLI: `go install github.com/swaggo/swag/cmd/swag@latest`
- gotext: `go install golang.org/x/text/cmd/gotext@latest`

## Updating the Go Version

1. Check the official Go release notes before bumping — they list any breaking changes.
2. Install the new Go version and set it as active.
3. Update `go.mod`: change the `go` directive to the new version.
4. Run `go mod tidy` — this syncs toolchain requirements and may add/update a `toolchain` line.

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

Two layers:

| Layer | Source | Requires restart |
|---|---|---|
| **AppConfig** | Environment variables | Yes |
| **Settings** | `storage/config/settings.json` | No |

**AppConfig** (`config.go`) — server-level options (paths, ports, intervals) loaded once at startup. Add a field to `AppConfig` and a `case` in `applyEnvToAppConfig`.

**Settings** (`settings_*.go`) — user preferences editable in the UI. Each setting is a typed package-level variable declared in `settings_registry.go` and registered at init time. Adding a setting there is all that's needed — persistence, UI rendering, and `MyNewSetting.Get()` access are automatic.

- Types: `*BoolSetting`, `*IntSetting`, `*StringSetting` (also renders as select or dynamic-select), `*StringSliceSetting`, `*MapSetting[T]` (structured/nested values, no UI)
- Sections and groups are declared in `settings_definitions.go` and appear in the UI automatically
- `OnChange` fires on API saves (`SetFromString`) but not on startup load — startup side-effects are applied explicitly in `InitSettings`
- `IntSetting` and `StringSetting` validate against `Min`/`Max` and `Options` respectively; invalid API values return 400, invalid stored values fall back to the default with a warning
- All values are stored in `atomic.Pointer[T]` — reads are lock-free. `MapSetting` uses copy-on-write: always build a fresh copy before calling `Set`, never mutate the map returned by `Get`

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

# Logging

Centralised logging in `internal/logging`. All app code uses the four level functions (`LogDebug`, `LogInfo`, `LogWarning`, `LogError`) — never the standard `log` package directly.

**Ring buffer**

- Every log entry is held in a fixed-size in-memory ring buffer (last 500 entries), regardless of level.
- Powers the in-app log viewer at `/system/logs` — live-polled via htmx, filterable, pauseable.

**File output**

- Opt-in via `KNOV_LOG_FILE_ENABLED` (default on). Writes to `logs/app.log` with size-based rotation.
- Level threshold controlled by `KNOV_LOG_FILE_LEVEL` (default `info`).
- A session separator is written to the file on every startup so restarts are immediately visible when scrolling.

**Standard library interception**

- `InitInterceptor()` wraps `log.SetOutput` so all third-party and framework `log.Printf` calls (e.g. chi access logs) are also captured — into the ring buffer and into the file.
- Must be called before any other initialisation to avoid missing early entries.

**Named loggers**

- `LogBuilder(key)` returns a `*log.Logger` that appends to `logs/<key>.log`. Used for long-running background jobs where a dedicated file is more useful than the main log.

**Env vars**

```
KNOV_LOG_LEVEL          # console threshold: debug | info | warning | error (default: info)
KNOV_LOG_FILE_ENABLED   # set to "false" to disable file output (default: on)
KNOV_LOG_FILE_LEVEL     # file threshold: debug | info | warning | error (default: info)
KNOV_LOG_MAX_SIZE_MB    # max size per log file in MB before rotation (default: 10)
KNOV_LOG_MAX_FILES      # number of rotated files to keep (default: 5)
KNOV_LOGS_PATH          # override the logs directory (default: ./logs)
```

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

- Board is a **page shell + HTMX** pattern: `/kanban/{board}` renders the template, `GET /api/kanban/{board}` returns the column HTML on load and on filter change. `{board}` is a URL slug, not a raw folder path.
- Excerpts are **lazy-loaded** per card via `GET /api/kanban/excerpt?filepath=...&chars=30`
- Card moves are **optimistic UI** — the card is moved in the DOM immediately, then `POST /api/kanban/card/move` persists the tag change using `MetaDataSaveRaw` (skips parent/link processing)

## Tag System

- Kanban state is stored as a regular metadata tag: `{prefix}-status-{status}` (e.g. `kb-status-inbox`)
- Prefix and valid statuses come from env: `KNOV_KANBAN_PREFIX`, `KNOV_KANBAN_STATUS`
- `sanitizeKanbanTags()` in `metadata.go` enforces: one kanban tag max, known sub-namespace only (`status` for now), status must be in allowlist — called on every `MetaDataSave`
- Adding a new sub-namespace (e.g. `kb-priority-*`): add it to `knownSubNamespaces` in `sanitizeKanbanTags`

## Boards

- Boards are explicitly configured folders, not auto-derived collections: `KNOV_KANBAN_BOARDS=folder/path:Display Name,...` (`internal/configmanager/config.go` → `AppConfig.KanbanBoards []KanbanBoard{FolderPath, DisplayName, Slug}`)
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

## Env Vars

```
KNOV_KANBAN_BOARDS=projects/work:Work Board,personal/todo:Personal Todo  # boards (folder:name)
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

Or via file extension — certain extensions map automatically: `.filter` → filter-editor, `.list` → list-editor, `.todo` → todo-editor, `.index` / `.moc` → index-editor, `.txt` → textarea-editor.

## build the codemirror editor


```bash
mkdir ~/codemirror-bundle && cd ~/codemirror-bundle
npm init -y
npm install @codemirror/state @codemirror/view @codemirror/commands @codemirror/search @codemirror/language @codemirror/lang-markdown @codemirror/autocomplete @codemirror/lint @replit/codemirror-vim @uiw/codemirror-extensions-line-numbers-relative
npm install --save-dev esbuild

vi editor.js
```

```js
import { EditorState } from "@codemirror/state";
import {
  EditorView,
  keymap,
  drawSelection,
  highlightActiveLine,
  placeholder,
  lineNumbers,
  highlightSpecialChars,
} from "@codemirror/view";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import {
  search,
  searchKeymap,
  highlightSelectionMatches,
} from "@codemirror/search";
import { vim } from "@replit/codemirror-vim";
import {
  bracketMatching,
  foldGutter,
  syntaxHighlighting,
  defaultHighlightStyle,
  foldKeymap,
  indentUnit,
} from "@codemirror/language";
import { markdown } from "@codemirror/lang-markdown";
import {
  autocompletion,
  completionKeymap,
  closeBrackets,
  closeBracketsKeymap,
} from "@codemirror/autocomplete";
import { lintKeymap } from "@codemirror/lint";

// Expose a single constructor on window so the app can call it
window.createCodeMirror = function (element, content, options) {
  options = options || {};

  var extensions = [
    // VIM - MUST BE FIRST for proper Vim mode behavior
    vim(),

    // Core editing features
    history(),
    drawSelection(),
    EditorView.lineWrapping,

    // Markdown language support with syntax highlighting
    markdown(),
    syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
    bracketMatching(),

    // Convenience features for Markdown editing
    closeBrackets(), // Auto-close brackets, quotes, etc.
    autocompletion(), // Word-based autocompletion
    indentUnit.of("  "), // Use 2 spaces for indentation

    // Visual enhancements
    highlightActiveLine(),
    lineNumbers(),
    lineNumbersRelative(),
    highlightSelectionMatches(),
    highlightSpecialChars(), // Shows invisible characters
    foldGutter(), // Code folding in gutter

    // Search functionality
    search(),

    // Keymaps (order matters for proper keybinding priority)
    keymap.of([
      ...defaultKeymap,
      ...historyKeymap,
      ...searchKeymap,
      ...completionKeymap,
      ...closeBracketsKeymap,
      ...foldKeymap,
      // ...lintKeymap, // Uncomment if you add linting later
    ]),

    // Update listener for change events
    EditorView.updateListener.of(function (update) {
      if (options.onChange) {
        options.onChange(update);
      }
    }),
  ];

  // Add placeholder if provided
  if (options.placeholder) {
    extensions.push(placeholder(options.placeholder));
  }

  // Create the editor state
  var state = EditorState.create({
    doc: content || "",
    extensions: extensions,
  });

  // Return the editor view instance
  return new EditorView({
    state: state,
    parent: element,
  });
};
```

```bash
npx esbuild editor.js --bundle --minify --outfile=codemirror6-bundle.min.js
ls -lh codemirror6-bundle.min.js
```

# Creating a Custom Theme

## Structure

A theme lives in `themes/<your-theme-name>/` and follows this layout:

```
themes/<name>/
├── theme.json          # metadata and settings schema
├── css/
│   └── style.css       # main stylesheet — loaded via /themes/<name>/css/style.css
├── js/
│   └── *.js            # theme-owned scripts
└── *.gohtml            # one file per required template
```

The `css/` and `js/` subdirectories are conventions — all files anywhere under `themes/<name>/` are served automatically. There is no registration step.

## theme.json

Defined by `ThemeMetadata` in `internal/thememanager/thememanager.go`. Required fields: `name`, `version`, `author`, `description`. Optional: `themeSettings` — a map of setting keys to `ThemeSetting` objects (type, default, label). Settings are exposed in the UI automatically and available in templates via `.ThemeSettings`.

Setting types supported: `boolean`, `select` (with `options`), `textarea`, `number` (with optional `min`/`max`).

## CSS Variables

`static/css/defaults.css` is injected automatically before any theme CSS and defines fallback values for every CSS variable the app uses. Themes override these on `body` (or more specific selectors) in their own stylesheet — any variable left unset falls back to the default, so themes only need to declare what they actually change.

Variables available: `--bg`, `--bg-secondary`, `--text`, `--text-secondary`, `--border`, `--surface`, `--primary`, `--accent`, `--neutral`, `--danger`, `--success`, `--warning`, and the four `--find-highlight-*` variables. Dark mode defaults are applied automatically via `body[data-dark-mode="true"]`.

## Templates

- Every template must define a `content` block consumed by `base.gohtml`
- `base.gohtml` is the layout shell — it must include the theme stylesheet, HTMX, and any other global scripts your theme needs
- Static editor assets (markdown editor, list editor, filter editor CSS/JS, etc.) are **injected automatically** before `</head>` and `</body>` — do not add them manually
- Template data structs are defined in `internal/server/thememanager/template_data.go` — reference these to know what fields are available per page
- Translation is available via the `T` template function

### Loading theme scripts

Use `{{.CurrentTheme}}` in `<script src>` and `<link href>` paths so the correct file is served regardless of the active theme:

```html
<link href="/themes/{{.CurrentTheme}}/css/style.css" rel="stylesheet"/>
<script src="/themes/{{.CurrentTheme}}/js/my-script.js"></script>
```

### Passing Go template data to JS files

External JS files cannot contain Go template directives. When a script needs server-side data, pass it through the HTML before loading the script.

**Simple values — use `data-*` attributes:**

```html
<input id="search-page-input" data-format="{{ index .ThemeSettings "searchResultsDisplay" }}">
<script src="/themes/{{.CurrentTheme}}/js/search-mode.js"></script>
```

```js
// search-mode.js
var format = document.getElementById('search-page-input').dataset.format || 'cards';
```

**Complex config — use an inline config object:**

```html
<script>
window.MY_CONFIG = {
    collection: '{{.Collection}}',
    label: '{{T "some label"}}'
};
</script>
<script src="/themes/{{.CurrentTheme}}/js/my-feature.js"></script>
```

```js
// my-feature.js
var cfg = window.MY_CONFIG || {};
var collection = cfg.collection || '';
```

Keep the inline block to pure data — no logic. All behaviour goes in the external file.

## Bundled Libraries

All vendor libraries are bundled into the binary and served from `/static/` — no CDN dependency, works fully offline.

### Automatically injected (do not add to your theme)

The thememanager injects these before `</head>` and `</body>` on every page. Theme authors must not add them manually:

| Asset | Path | Purpose |
|---|---|---|
| notify.css | `/static/css/notify.css` | Toast notification styles |
| codehighlight.css | `/static/css/codehighlight.css` | Syntax highlight styles for `<pre><code>` blocks |
| codemirroreditor.css | `/static/css/codemirroreditor.css` | CodeMirror editor component styles |
| toastuieditor.css | `/static/css/toastuieditor.css` | ToastUI editor component styles |
| indexeditor.css | `/static/css/indexeditor.css` | Index/MOC editor styles |
| listeditor.css | `/static/css/listeditor.css` | List editor styles |
| todoeditor.css | `/static/css/todoeditor.css` | Todo editor styles |
| tableeditor.css | `/static/css/tableeditor.css` | Table editor styles |
| filtereditor.css | `/static/css/filtereditor.css` | Filter editor styles |
| kanban.css | `/static/css/kanban.css` | Kanban board styles |
| notify-toast.js | `/static/notify-toast.js` | Toast notification display — required because the server emits `HX-Trigger: notify` headers on every action. The thememanager also injects the `#component-notify` container div with a `data-duration` attribute; do not add either manually. |
| wiki-autocomplete.js | `/static/wiki-autocomplete.js` | Wiki link autocomplete in editors |
| todo-state.js | `/static/todo-state.js` | Todo checkbox state persistence |
| conflict-diff.js | `/static/conflict-diff.js` | Conflict diff visualisation |

### Available in your base.gohtml

These are loaded by the built-in themes and available for use. If your theme needs any of them, include the `<link>` or `<script>` tag in your `base.gohtml`:

| Library | JS | CSS | Purpose |
|---|---|---|---|
| htmx 2.0.10 | `/static/htmx-2.0.10.min.js` | — | AJAX/hypermedia — required for all dynamic content |
| Font Awesome 7 | — | `/static/font-awesome-7.0.1-all.min.css` | Icon set (solid, regular, brands) |
| ToastUI Editor 3.2.2 | `/static/toastui-editor-3.2.2.min.js` | `/static/toastui-editor-3.2.2.min.css` | Markdown WYSIWYG editor (used by the default markdown editor) |
| CodeMirror 5.6.65.7 | `/static/codemirror-5.6.65.7.min.js` | `/static/codemirror-5.6.65.7.min.css` | Code/text editor (used by the codemirror editor) |
| CodeMirror Vim keymap | `/static/codemirror-vim.5.6.65.7.min.js` | — | Vim keybindings for CodeMirror |
| Handsontable 16.2.0 | `/static/handsontable-16.2.0.full.min.js` | `/static/handsontable-16.2.0.full.min.css` `/static/handsontable-theme-main-16.2.0.min.css` | Spreadsheet grid (used by the table editor) |
| SortableJS 1.15.0 | `/static/sortable-1.15.0.min.js` | — | Drag-and-drop sorting (used by list/kanban editors) |

htmx is effectively required. All others are only needed if your theme renders the corresponding editor or component.

### Theme-owned scripts

Each built-in theme ships its own JS files alongside the templates. These are **not** injected automatically — each theme owns and loads them explicitly. When building a custom theme, write your own versions (or copy and adapt the built-in ones as a starting point).

| Script | Purpose | Where to load | Notes |
|---|---|---|---|
| `copy-code.js` | Adds a "copy" button to every `<pre>` block; re-runs on `htmx:afterSettle` | `base.gohtml` | |
| `chat-bulk.js` | Bulk select/delete/move for chat messages | `base.gohtml` if the theme has a global chat panel; `chat.gohtml` otherwise | Exposes `chatBulkSubmit`, `chatBulkDelete` etc. as globals |
| `search-mode.js` | `updatePageSearchMode()` — rebuilds the `hx-get` URL when search options change | `search.gohtml` | Requires `data-format` attribute on `#search-page-input` |
| `history-search.js` | `updateLatestChangesSearch(query)` — debounced search on the history/latest-changes page | `history.gohtml` | Requires `data-collection` attribute on `#latestchanges-results` |
| `fileview-reader.js` | `adjustFontSize(delta)` — font size control in reader mode | `fileview.gohtml` reader section only | |
| `kanban.js` | Full kanban drag-and-drop and events panel | `kanban.gohtml` | Requires `window.KANBAN_CONFIG` set inline before this script; see built-in `kanban.gohtml` for the config shape |
| `nav-setup.js` *(builtin only)* | Wires edit/rename/delete modal actions to the current URL; positions popovers; `openNotificationsPopover()` | end of `base.gohtml` `edit` template block | Reads `window.location.pathname` — no template data needed |

## CSS Conventions

- Use CSS custom properties (`--primary`, `--bg`, `--text`, `--border`, etc.) for colors so the theme responds to dark mode and color scheme settings
- Global styles go in `style.css`; page/component-specific styles should use ID selectors (`#page-*`, `#component-*`, `#view-*`) to avoid conflicts with injected editor styles
- Dark mode is driven by `data-dark-mode="true"` on `<body>`; color scheme by `data-color-scheme`

## Loading

- Themes are loaded from `themes/` at startup by `internal/thememanager/thememanager.go` => `loadAllThemes()`
- The folder name becomes the theme identifier used in settings
- `themes/overwrite/` is reserved and skipped during theme discovery
- Switch themes via **Settings => Theme** — no restart required

## System Pages

There are a few system pages e.g. `/system/changelogs`,`/system/health` or `/system/version` which are automatically created by the binary - they can be styled using these classes:

- `.system-page` — outer wrapper
- `.system-page-title` — the page `<h1>`
- `.system-page-content` — content area
- `.system-page-toc` — TOC nav (same as `.toc-nav` used elsewhere)

# System Pages

`/system/*` is a namespace for app-internal pages whose **content is controlled by the application**, not theme templates.

**How it works**

- `ThemeManager.RenderSystemPage(w, title, content)` renders a page using the current theme's `base.gohtml`
- the content block is an inline constant in `internal/thememanager/system.go`
- nothing theme authors can override

## /system/changelog

Renders all changelog markdown files from `docs/changelogs/` merged and sorted newest-first, displayed in the standard file view layout.

## /system/logs

Live in-app log viewer. Polls the in-memory ring buffer every 5 seconds. Supports client-side text filter, pause toggle, and — when file logging is active — a full-file view and download link.

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
