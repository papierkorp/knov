# Testing

In-app runtime test suites - not `go test`. Knov ships as a single binary with no go toolchain on the target machine, so tests need to be runnable against a real running instance, from an admin button or an API call. The filter suite (`internal/test/filtertest/`, wired to the admin page and `POST /api/testdata/filtertest`) is the model every other suite follows.

**Suite interface**
- `internal/test` defines the shared shape every suite returns: `CaseResult` (name, free-form `Expected`/`Actual` strings, error, success, `Detail any` for suite-specific extras) and `SuiteResult` (suite name, totals, pass/fail, list of `CaseResult`), plus a `Suite` interface (`Name() string`, `Run() (*SuiteResult, error)`)
- `Expected`/`Actual` are plain strings rather than typed values, since suites compare very different things (a list of matching files, a single pass/fail, rendered content) - each suite formats its own comparison text

**Package layout**
- One subpackage per test group under `internal/test/`, e.g. `internal/test/filtertest`, `internal/test/editorstest` - each seeds real files/metadata via the internal packages directly (no HTTP round-trip) and implements `Suite`
- Subpackages are always suffixed `test` (`filtertest`, not `filter`) - a subpackage named `filter` would collide with `knov/internal/filter` in every file that needs both (job wrapper, API handler), forcing an import alias everywhere; the suffix avoids that
- `internal/test/registry.go` holds `RunAllTests()`, which runs the registered suites in order and aggregates. Suites self-register via `test.Register(Suite{})` in their own `init()` (a `<group>test` package importing `internal/test` for the shared types rules out `internal/test` importing back to build the list directly) - adding a suite later means adding its subpackage plus that `init()` line
- Every suite's sample files live under `docs/test/` (e.g. `test/filter-tests`, `test/editors-tests`) so the admin "Clean Test Data" button removes them all in one go
- Same file layout in every subpackage: `<group>test.go` holds only the `Suite` type (`Name()`, `Run()`); `sampledata.go` holds the setup - physical file writes, metadata, git commit helpers, wipe/reseed; `testcases.go` (or `testcases_<category>.go` when there's enough of them to split) holds the actual cases

**Wiring (same shape for every suite, including `RunAllTests()` itself)**
- Every suite self-registers via `job.RegisterSuiteRunner(name, run)` in its own `init()`, alongside its `test.Register(Suite{})` call - `internal/job/externalsuite.go`'s `externalSuiteJob` looks the runner up by name at execute time (mutex-guarded, recorded in job history, visible at `/system/jobs`), so `internal/job` never imports a suite package directly. That's what lets a suite that itself needs to call into `internal/job` (e.g. jobstest, to exercise the scheduler/history directly) exist without an import cycle
- An HTTP handler in `internal/server`, swagger-annotated, at `POST /api/testdata/<group>test`
- A button on the admin page

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
- `kanban.MoveCard` saves via `MetaDataSaveRaw`, which skips the cache refresh `MetaDataSave` normally triggers - cases call `files.InvalidateFileListCache()` afterward so `BuildBoard` sees the move immediately (the kanban analog of searchtest's synchronous reindex)
- Sample cards pin `CreatedAt` via a `MetaDataSave` followed by a `MetaDataGet`+`MetaDataSaveRaw` round-trip, since `MetaDataSave` always stamps `LastEdited`/`CreatedAt` from the save call itself
- `MetaDataSave` only overwrites `Tags` when the new value is non-empty, so a stale kanban status tag from a previous run has to be stripped explicitly via `MetaDataSaveRaw` at seed time
- Column order (`kanban-order/<folder>`) is config-store backed like dashboards, not touched by wiping `docs/test/`, so it's reset at suite start and via `defer`
- Native HTML5 drag-and-drop itself is the one piece genuinely untestable outside a browser - the suite covers the API/state it drives (`SaveOrder`/`BuildBoard`) instead

## Browse suite (`internal/test/browsetest`)
- Calls `internal/files`' tree/folder/browse/autocomplete functions directly, replicating each handler's inline logic (file-tree nesting, folder-contents listing, browse-by-tag/folder, autocomplete, folder suggestions, header/TOC extraction) since none of it is exported as a single callable
- Browse-by-folder needs a folder *segment* as the query value, not the joined path - `Folders` is stored one path segment per element
- `GetAllFolderPathsFromCache` only refreshes in a background goroutine after `MetaDataSave`, so the suite calls `files.RebuildAllCaches()` synchronously to avoid racing it

## Metadata suite (`internal/test/metadatatest`)
- Calls `internal/files`' metadata get/set/delete/export functions directly, covering every settable field and the partial-update semantics (empty `Tags`/`Editor` means "unspecified", not "clear it")
- References add/remove has no exported wrapper, so the suite replicates the handler's inline append/filter
- `MetaDataSave` only overwrites `References` when non-nil, so `resetAndSeed` explicitly deletes metadata first rather than relying on a physical-file wipe

## Connections suite (`internal/test/connectionstest`)
- Seeds parents/kids/ancestors and used-links/links-to-here via real `MetaDataSave`/`UpdateLinksForSingleFile` calls so the actual cascade computes them, not faked
- Grandchildren replicates the handler's inline kid-of-kids loop, since there's no exported equivalent
- Related-files has no per-file computation path (only a full-vault rebuild computes it), so it's seeded directly via `MetaDataSaveRaw` instead

## Jobs suite (`internal/test/jobstest`)
- Calls `job.RunFullRebuild`/`RunSearchReindex`/`RunCacheInvalidate`/`RunMediaCleanup` and the manual "run all jobs" trigger directly, asserting on the resulting filesystem/DB state rather than just "no error" - e.g. seeding a raw save that bypasses the normal link cascade, so `Ancestor`/`Kids`/`UsedLinks` only exist once the rebuild job actually recomputes them
- The first suite that itself needs to call into `internal/job` (to exercise the scheduler/history) while also needing its own admin-button job - see the `RegisterSuiteRunner` wiring above, which this suite drove the design of

## Media suite (`internal/test/mediatest`)
- Calls `files.UploadMedia` with a real `multipart.File`/`*multipart.FileHeader` (round-tripped through an actual multipart form body), and the other media functions (list/partition, delete, storage stats) directly
- Rename and orphan-detection replicate their handlers' inline sequence, since neither has an exported wrapper
- Leaves the orphaned-media *cleanup* job itself to jobstest, to avoid duplicating that one case across two suites

## Export/import suite (`internal/test/exporttest`)
- Both zip-export handlers are inline `filepath.Walk`+`archive/zip` logic with no exported wrapper, so the suite replicates the walk directly, round-tripping through a real `zip.Writer`/`zip.Reader`
- Settings export/import round-trips a real setting (`HideTodo`) through `configmanager.ExportSettingsJSON`/`ImportSettingsJSON`, restored via `defer` since it's a real global setting, not sandboxed test data

## Notification suite (`internal/test/notificationstest`)
- Calls `internal/notificationStorage`'s exported API directly (`Add`/`ConsumePending`/`GetRecent`/`DeleteByID`/`Clear`) - everything here is already exported, no handler logic to replicate
- Notifications are a real global sqlite log, not tied to `docs/test/`, so cases self-clean themselves rather than relying on the folder wipe
- `Clear()` is the real "Clear all" admin action and has no undo - the case accepts that outcome, same as a user clicking the real button would

## Settings suite (`internal/test/settingstest`)
- Calls `configmanager`'s settings registry (`BulkSetFromForm`, `GetSetting(key).SetFromString`) and `thememanager`'s theme list/switch/settings directly - almost everything here is exported, the one exception being favicon upload/delete's inline file write
- Every case mutates a real persisted global setting (not sandboxed test data) and restores the original value via `defer`, same pattern as exporttest's settings round-trip
- Config repo url isn't re-tested here - it's the same `UpdateEnvFile`+`git.EnsureRemote` pair githistorytest's remote case already exercises; data path change stays out of scope entirely (needs a process restart to take effect)

## Logs suite (`internal/test/logstest`)
- Calls `logging.GetRecentEntries` directly for the ring buffer, and replicates `handleAPIGetLogsFile`'s inline offset/limit slicing arithmetic for pagination/chunking, since there's no exported wrapper for it
- Reuses `logging.KeyInAppTests` - already a real, shared log key the job scheduler logs every suite run's summary to - rather than inventing a synthetic key
- Assertions check that probe lines appear in the expected region (substring containment) rather than exact byte/line-count equality, tolerating real interleaved log activity

## Parser suite (`internal/test/parsertest`)
- Calls `parser.ProcessMarkdownLinks` directly - pure string-in/string-out, no file IO and no global state, the only suite with no setup/teardown at all
- Covers empty-link-text fallback labels, percent-encoded path/anchor segments decoded before the fallback label is built (the href's anchor fragment itself is left exactly as originally written), unicode header slug capitalization, and external links/image embeds left untouched
- `humanizeSlug` stays unexported and is fully exercised through `ProcessMarkdownLinks`, so no replication was needed here, unlike every other suite
