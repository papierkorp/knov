# Testing

Two kinds of tests live in this repo, split by whether a case needs a real running app instance:

- **In-app runtime test suites** (`internal/test/<x>test`, run via `knov --start-tests`) - for anything that depends on live data/storage, real installed theme files on disk, or other state a sandboxed `go test` can't cheaply fake. Knov ships as a single binary with no go toolchain on the target machine, so these need to be runnable against a real built binary: `knov --start-tests` runs every suite headless against an isolated `knov_temp_test` copy of the live data/storage directories (see `test.PrepareIsolatedStorage`/`test.RunAllTestsAndLog`, called from main.go), then exits - it's a separate process from any `knov` instance already running, so it never touches live data. Logging follows the same split: `logging.SetIsolatedLogsDir` (called from main.go before `configmanager.InitAppConfig`) redirects every log key except `KeyInAppTests` into `knov_temp_test/logs`, so the incidental logging from exercising every suite doesn't land in live log files - `KeyInAppTests` still writes to the real logs directory, giving a single persistent history of every test run, live or isolated. The filter suite (`internal/test/filtertest/`) is the model every other suite follows.
- **Plain `go test` files** colocated in the package they cover - for pure logic and anything that only needs a package-level backend (e.g. `configStorage`) that a `TestMain` can point at a throwaway `os.MkdirTemp` dir, with no dependency on real installed files or a live app instance. See "Plain `go test` packages" below.

**Suite interface**
- `internal/test` defines the shared shape every suite returns: `CaseResult` (name, free-form `Expected`/`Actual` strings, error, success, `Detail any` for suite-specific extras) and `SuiteResult` (suite name, totals, pass/fail, list of `CaseResult`), plus a `Suite` interface (`Name() string`, `Run() (*SuiteResult, error)`)
- `Expected`/`Actual` are plain strings rather than typed values, since suites compare very different things (a list of matching files, a single pass/fail, rendered content) - each suite formats its own comparison text

**Package layout**
- One subpackage per test group under `internal/test/`, e.g. `internal/test/filtertest`, `internal/test/editorstest` - each seeds real files/metadata via the internal packages directly (no HTTP round-trip) and implements `Suite`
- Subpackages are always suffixed `test` (`filtertest`, not `filter`) - a subpackage named `filter` would collide with `knov/internal/filter` in every file that needs both, forcing an import alias everywhere; the suffix avoids that
- `internal/test/registry.go` holds `RunAllTests()`, which runs the registered suites in order and aggregates. Suites self-register via `test.Register(Suite{})` in their own `init()` (a `<group>test` package importing `internal/test` for the shared types rules out `internal/test` importing back to build the list directly) - adding a suite later means adding its subpackage plus that `init()` line, plus a blank import of it in main.go so that `init()` actually runs
- Every suite's sample files live under `docs/test/` (e.g. `test/filter-tests`, `test/editors-tests`) so the admin "Clean Test Data" button removes them all in one go
- Same file layout in every subpackage: `<group>test.go` holds only the `Suite` type (`Name()`, `Run()`); `sampledata.go` holds the setup - physical file writes, metadata, git commit helpers, wipe/reseed; `testcases.go` (or `testcases_<category>.go` when there's enough of them to split) holds the actual cases

**Wiring**
- `knov --start-tests` is the only entry point - main.go calls `test.PrepareIsolatedStorage()` before any storage backend initializes, then `test.RunAllTestsAndLog(name)` once they're all up, and exits. `knov --start-tests` runs every registered suite; `knov --start-tests <suite>` (e.g. `filter`) runs only the suite whose `Name()` matches - there's no admin button or API route for it, only the CLI
- Adding a suite means: its subpackage, its `test.Register(Suite{})` call, and a blank import of the subpackage in main.go - nothing in `internal/job` or `internal/server` needs to know about it

**Where `internal/testkit` fits**
- `internal/testkit` (`httptest` + `chromedp`) is not the primary vehicle for suites - it stays around for the rare case a suite genuinely needs a real HTTP/router pass, and for the handful of things an in-app suite structurally can't verify: real browser/JS interaction like kanban drag-and-drop or the toastui editor toolbar
- For those, cover the underlying API/state through a normal suite, and only reach for `testkit`'s chromedp path if the interaction itself needs checking

**Scope**
- The suite build order and the htmx/JS call inventory backing it live in `docs/temp_todo.md` under `# testing`

## Filter suite (`internal/test/filtertest`)
- Seeds a fixed set of test files and metadata, then runs a table of `filter.Config` scenarios directly against `filter.FilterFilesWithConfig` and compares the matched files to what's expected
- One case per scenario - covers logic combinations, each operator, include/exclude, parent/child/ancestor relations, references, and date comparisons

## Editors suite (`internal/test/editorstest`)
- Wipes and reseeds its own sample folder at the start of every run, then runs one independent case per editor operation: create+edit+save for every editor type, section save, table save, todo-toggle, convert-to-markdown, file rename/move, and the bulk ops (delete, metadata patch, chat move/delete)
- Editor HTTP handlers mix request parsing with business logic inline, so there's usually no single function to call directly - cases instead call the same underlying functions the handler calls (content storage write + metadata save + link rebuild, the content handler's section/table save, todo state cycling, the dokuwiki converter, etc.), reproducing the handler's real sequence of calls without an HTTP round-trip
- Two bulk-op cases (metadata patch, chat move) can't reach their handler's actual logic because it's unexported in `internal/server` - those replicate the same behavior using the equivalent exported building blocks instead

## Search suite (`internal/test/searchtest`)
- Seeds a few files (title match, content match, added-then-deleted) and calls `search.SearchFiles*`/`search.SearchDeletedFiles*` directly
- Indexes synchronously after seeding, since content search otherwise depends on the periodic reindex cronjob
- Doesn't cover the response-format rendering (dropdown/list/cards) - `internal/server/render` imports `internal/job`, which imports every suite, so importing it here would cycle

## Git history suite (`internal/test/githistorytest`)
- Seeds a versioned file and an added-then-deleted file, committed via git, then calls `internal/git`'s history/diff/restore/remote functions directly
- Collection filtering checks inclusion under the shared `test` collection and exclusion under a made-up collection name, since collection is derived from a file's top-level folder - nesting sample files under `docs/test/` means every suite's files share that one real collection, so distinct real collections can't be told apart here
- The remote case points the git remote at a throwaway local bare repo (no network) and always restores whatever was configured before it ran

## Chat suite (`internal/test/chattest`)
- Calls `internal/chat`'s exported single-message API directly (add/delete/get-by-id, `GetPage` pagination, `MoveFilePath`, `DeleteForFile`) for both global and file-scoped messages
- `handleAPIMoveChatMessage`/`handleAPIBulkMoveChatMessages`/`handleAPIBulkDeleteChatMessages`/`formatForEditor` are unexported in `internal/server`, so the move/bulk-move/bulk-delete cases replicate their exact call sequence instead - same approach as editorstest's bulk-metadata-patch case
- Global (unscoped) messages aren't tied to a file path, so they can't be cleared by wiping the suite's `docs/test/` folder like every other suite's sample data - cases that create global messages delete them again themselves, and cases using fixed file-scoped paths clear those via `DeleteForFile` both at suite start and via `defer`

## Dashboard suite (`internal/test/dashboardtest`)
- Calls `internal/dashboard`'s exported CRUD directly, and covers each widget type's underlying data resolution (filter, fileContent, tags/collections/folders) rather than rendered HTML - `render.RenderWidget` lives in `internal/server/render`, unreachable here for the same import-cycle reason noted for search's format rendering
- Export/import is a trivial `json.MarshalIndent`/`Unmarshal` round-trip in the real handler, replicated inline rather than imported
- Dashboards live in `configStorage` keyed by id, not under `docs/test/` - fixed dashboard names are deleted by their derived id at suite start instead of relying on a folder wipe

## Kanban suite (`internal/test/kanbantest`)
- Calls `internal/kanban`'s exported board-build, card-move, order-persistence and helper functions directly
- `kanban.MoveCard` uses `MetaDataMutate` and calls `RefreshCaches` itself
- Sample cards pin `CreatedAt` via `test.SeedMetadata` (Sync + SetCreatedAt)
- Stale kanban status tags from a previous run are cleared by wiping metadata in `resetAndSeed` before re-seeding
- Column order (`kanban-order/<folder>`) is config-store backed like dashboards, not touched by wiping `docs/test/`, so it's reset at suite start and via `defer`
- Native HTML5 drag-and-drop itself is the one piece genuinely untestable outside a browser - the suite covers the API/state it drives (`SaveOrder`/`BuildBoard`) instead

## Browse suite (`internal/test/browsetest`)
- Calls `internal/files`' tree/folder/browse/autocomplete functions directly, replicating each handler's inline logic (file-tree nesting, folder-contents listing, browse-by-tag/folder, autocomplete, folder suggestions, header/TOC extraction) since none of it is exported as a single callable
- Browse-by-folder needs a folder *segment* as the query value, not the joined path - `Folders` is stored one path segment per element
- `GetAllFolderPathsFromCache` only refreshes in a background goroutine after Sync/Set*, so the suite calls `files.RebuildAllCaches()` synchronously to avoid racing it

## Metadata suite (`internal/test/metadatatest`)
- Calls `internal/files`' metadata get/set/delete/export functions directly, covering every settable field and the partial-update semantics (empty `Tags`/`Editor` means "unspecified", not "clear it")
- References add/remove has no exported wrapper, so the suite replicates the handler's inline append/filter
- `resetAndSeed` explicitly deletes metadata first rather than relying on a physical-file wipe, so leftover user fields cannot survive across runs

## Connections suite (`internal/test/connectionstest`)
- Seeds parents/kids/ancestors and used-links/links-to-here via real `SeedMetadata`/`SetParents`/`UpdateLinksForSingleFile` calls so the actual cascade computes them, not faked
- Grandchildren replicates the handler's inline kid-of-kids loop, since there's no exported equivalent
- Related-files has no per-file computation path (only a full-vault rebuild computes it), so it's seeded directly via `SeedMetadataRaw` instead

## Jobs suite (`internal/test/jobstest`)
- Calls `job.RunFullRebuild`/`RunSearchReindex`/`RunCacheInvalidate`/`RunMediaCleanup` and the manual "run all jobs" trigger directly, asserting on the resulting filesystem/DB state rather than just "no error" - e.g. seeding a raw save that bypasses the normal link cascade, so `Ancestor`/`Kids`/`UsedLinks` only exist once the rebuild job actually recomputes them
- The first suite that itself needs to call into `internal/job` (to exercise the scheduler/history) - this is why suites are wired via a blank import + `test.Register` rather than `internal/job` importing suite packages directly, which would cycle

## Media suite (`internal/test/mediatest`)
- Calls `files.UploadMedia` with a real `multipart.File`/`*multipart.FileHeader` (round-tripped through an actual multipart form body), and the other media functions (list/partition, delete, storage stats) directly
- Rename and orphan-detection replicate their handlers' inline sequence, since neither has an exported wrapper
- Leaves the orphaned-media *cleanup* job itself to jobstest, to avoid duplicating that one case across two suites

## Export/import suite (`internal/test/exporttest`)
- Both zip-export handlers are inline `filepath.Walk`+`archive/zip` logic with no exported wrapper, so the suite replicates the walk directly, round-tripping through a real `zip.Writer`/`zip.Reader`
- Settings export/import round-trips a real setting (`HideTodo`) through `configmanager.ExportSettingsJSON`/`ImportSettingsJSON`, restored via `defer` since it's a real global setting, not sandboxed test data

## Settings suite (`internal/test/settingstest`)
- Only theme list/switch are left here (`thememanager.GetThemeManager` loads `themes/<name>/theme.json` off `configmanager.GetThemesPath()`, which only exists against a real app instance)
- Everything else that used to be here (plain settings, favicon, hidePaths, languages, theme *settings* persistence) only ever needed `configStorage`, and has moved to `internal/configmanager/configmanager_settings_test.go` - see "Plain `go test` packages" below

## Logs suite (`internal/test/logstest`)
- Calls `logging.GetRecentEntries` directly for the ring buffer, and replicates `handleAPIGetLogsFile`'s inline offset/limit slicing arithmetic for pagination/chunking, since there's no exported wrapper for it
- Reuses `logging.KeyInAppTests` - already a real, shared log key `test.RunAllTestsAndLog` logs every `--start-tests` run's summary to - rather than inventing a synthetic key
- Assertions check that probe lines appear in the expected region (substring containment) rather than exact byte/line-count equality, tolerating real interleaved log activity

## Plain `go test` packages
Cases with no dependency on real installed files or a live app instance live as ordinary `_test.go` files in the package they cover, run with `go test ./...` like any Go project - no `--start-tests` involved.

- `internal/parser/parser_links_test.go`, `parser_headings_test.go` - `ProcessMarkdownLinks`, wikilink target/anchor resolution, and the heading scanner (including the end-to-end guard that pre-render scan ids match the rendered `<hN id="...">` ids)
- `internal/markdown/markdown_test.go` - the fence-aware scanning primitives (`FenceMask`, `CodeBlocks`, `StripFencedBlocks`)
- `internal/notificationStorage/notificationStorage_test.go` - `Add`/`ConsumePending`/`GetRecent`/`DeleteByID`/`Clear`, against a `TestMain`-initialized sqlite db in an `os.MkdirTemp` dir instead of the real global notification log
- `internal/configmanager/configmanager_settings_test.go` - the settings registry (`BulkSetFromForm`, `GetSetting(key).SetFromString`), languages, hidePaths validation, favicon upload/delete, and theme *settings* (`GetThemeSetting`/`SetThemeSetting`, `configStorage`-backed only, no real `theme.json` needed) - all against a `TestMain`-initialized `configStorage` in an `os.MkdirTemp` dir instead of the real live config

When adding a new case: if it only needs a package-level backend a `TestMain` can point at a temp dir, add it here; if it needs real installed files (themes, live data) or the full app wired up, it belongs in an `internal/test/<x>test` suite instead.
