# small stuff

**manual**
- add a smoketest todo file to testfiles
  - create a new file for each editor
  - move a file for each editor
  - edit a file for each editor
  - go to /kanban and move a task around
  - use the filterForm
  - create a dashboard with different widgets
  - browse media
  - use both builtin and rail theme
- translations
- add a `create_your_own_theme.md` file


**per ai**
- deployment
  - make docker build viable
    - for usage
    - for devs
- larger project if time, not important
  - codemirror copy + paste with y+p does not work properly (e.g. i have something in the clipboard and it doesnt paste and i need to use ctlr+v in edit mode)
    - is a harder problem to tackle
  - backup solution
  - create a system for themes (another repoistory with themes)
    - .e.g. create a table/dict with all top level folders - than check if there is a theme.json
- move file button (can be done in rename)
- kanban add button - let me set a folder e.g. i have a `sirconic` collection but i want new files to be created in `sirconic/todo` instead (allow me to set a folder for each collection)
- <details> are not shown and are blocking section edit

# testing

In-app runtime test suites, not `go test`. Knov ships as a single binary with no go toolchain on the target machine, so the existing pattern (`internal/test/testfilter.go`: seeds real files/metadata, runs real filter configs, returns pass/fail results, wired to an admin button + `POST /api/testdata/filtertest`) is the model to extend, not `go test`/httptest.

**architecture**
- shared interface in `internal/test`:
  ```go
  type CaseResult struct { Name, Expected, Actual, Error string; Success bool; Detail any }
  type SuiteResult struct { Suite string; Total, Passed, Failed int; Success bool; Cases []CaseResult }
  type Suite interface { Name() string; Run() (*SuiteResult, error) }
  ```
  `Expected`/`Actual` are free-form strings (not typed lists) - each suite formats its own comparison text, since groups compare very different things (file lists vs single pass/fail vs rendered content). `Detail` carries whatever group-specific extra the admin UI wants to show (e.g. the `filter.Config` used for a case).
- one subpackage per test group under `internal/test/`, e.g. `internal/test/filtertest`, `internal/test/editorstest`, `internal/test/chattest`. Suffix every subpackage with `test` (not the bare domain name) - a subpackage literally called `filter` would collide with `knov/internal/filter` in every file that needs both (cronjob.go, api handlers), forcing aliases everywhere. `test<group>` avoids that entirely.
- `internal/test/registry.go` holds `RunAllTests() (*SuiteResult, error)`, looping over a package-level `suites []Suite` and aggregating. `registry.go` can't construct `filtertest.Suite{}` etc. directly - every subpackage imports `internal/test` for the shared types, so `internal/test` importing back would cycle. Instead each `<group>test` package self-registers via `test.Register(Suite{})` in its own `init()`; that fires as soon as anything imports the subpackage (which its job wrapper in `internal/job` already does). Adding a group later is its subpackage plus that one `init()` line.
- per-suite wiring, same shape as the existing filter test: a `job.Job` wrapper in `internal/job` (mutex + history, `execute()`), an HTTP handler in `internal/server` (`POST /api/testdata/<group>test`, swagger-annotated), and an admin button. `RunAllTests()` gets the same treatment (its own job wrapper + `POST /api/testdata/run-all` + "run all tests" button).
- `internal/testkit` (httptest+chromedp harness from step 0) is not the primary vehicle for any of this - keep it around only for the rare case a group genuinely needs a real HTTP/router pass or actual browser JS check.
- same file layout in every `<group>test` package: `<group>test.go` holds only the `Suite` type (`Name()`, `Run()`); `sampledata.go` holds the setup (physical file writes, metadata, git commit helpers, wipe/reseed); `testcases.go` (or `testcases_<category>.go` when there's enough to split, as in editorstest) holds the actual cases
- every suite's sample files live under `docs/test/` (e.g. `test/filter-tests`, `test/editors-tests`) so the admin "Clean Test Data" button removes them all in one go - since collection is derived from a file's top-level folder, this means every suite's sample files share the same `test` collection (no distinct real collections to filter between within a single suite)

**0. build the architecture + rewrite filtertest as suite #1 (do first, step by step)** — done
- [x] add `Suite`/`SuiteResult`/`CaseResult` types to `internal/test` (new file, e.g. `internal/test/suite.go`)
- [x] create `internal/test/filtertest/` package; move `testfilter_testfiles.go` + `testfilter_testmetadata.go` into it unchanged (just package rename, fixtures stay as-is)
- [x] rewrite `testfilter.go` into `internal/test/filtertest/filtertest.go`: `CaseResult`/`SuiteResult` instead of `FilterTestResult`/`FilterTestResults`, add a `Suite`-implementing type (`Name() string { return "filter" }`, `Run()` wraps the existing `testConfigs` table, one `CaseResult` per config), delete the old `internal/test/testfilter.go`
- [x] update every caller of the old types: `internal/job/cronjob.go` (`filterTestJob`, `RunFilterTest`), `internal/job/scheduler.go` wrapper, `internal/server/api_tests.go` (`handleAPIFilterTest`, `handleAPIFilterTestMetadata`) + swagger annotations
- [x] update `render.RenderFilterTestResults` (or replace with a generic `SuiteResult` table renderer reusable by every future suite) so the admin page still renders correctly — replaced with `render.RenderSuiteResult`
- [x] add `internal/test/registry.go`: `suites []Suite` + `RunAllTests() (*SuiteResult, error)` — `internal/test` can't import its own `filtertest` subpackage (cycle, since `filtertest` imports `internal/test` for the shared types), so suites self-register via `test.Register()` in their `init()` instead of being listed directly in `registry.go`
- [x] wire `RunAllTests()` through its own job wrapper + `POST /api/testdata/run-all` + admin "run all tests" button
- [x] manual check: admin page "run filter tests" button and new "run all tests" button both work against a real running dev instance, endpoint/button locations unchanged (no user-facing change) — verified via curl against `go run ./`: `/api/testdata/filtertest`, `/api/testdata/filtertest/testdata`, and `/api/testdata/run-all` all return correct HTML (18/18 passed), admin page renders all three buttons

**suite build order** (after step 0; filter is done as part of step 0, so this starts at the next group)
- [x] 1. editors - create+edit+save for toastui, textarea, codemirror, filter, list, todo, index, table; section-save, table-save, todo-toggle, convert-to-markdown; file rename/move (updates links elsewhere); bulk ops (bulk delete files, bulk metadata patch, bulk chat move/delete) - `internal/test/editorstest`, 16 cases, wired via `job.RunEditorsTest`/`POST /api/testdata/editorstest`/admin button; verified live (16/16 passed, run-all shows 34/34 across both suites). Handlers mix HTTP parsing with business logic inline (no clean extraction point for most saves), so cases replicate the handler's exact sequence of calls (contentStorage.WriteFile/os.Rename + files.MetaDataSave/UpdateLinksForMovedFile/UpdateLinksForSingleFile, contentHandler.SaveSection/SaveTable, parser.CycleTodoStateAtLine, dokuwikiconverter) rather than calling the handlers themselves. bulk-metadata-patch replicates unexported `applyBulkPatch`'s tag-add logic directly (can't import it - lives in package `server`); bulk-chat-move covers the handler's "append" mode only (skips `formatForEditor`, also unexported). Avoided importing `internal/server/render` for `ListItem`/markdown-converter helpers since `render` imports `internal/job` (cycle back through the suite's job wrapper) - list/todo markdown built by hand instead.
- [x] 2. search & history - split into two suites since search and git history are different concerns:
  - search (title-only, full-content, deleted-file/history, empty query, limit truncation) - `internal/test/searchtest`, 6 cases, wired via `job.RunSearchTest`/`POST /api/testdata/searchtest`/admin button. Seeds alpha (title match), beta (content match) and delta (added then deleted, for deleted-file search) in `test/search-tests`, committed via git. Calls `search.IndexAllFiles()` synchronously after seeding since full-content search only ever sees what the periodic reindex cronjob last indexed. Skipped testing the render-format dispatch (dropdown/list/cards) directly - `internal/server/render` imports `internal/job`, which imports every suite's job wrapper, so importing it from a suite package would cycle; covered the shared limit-truncation behavior behind all formats instead.
  - git repo history (latest-changes pagination, filter by collection, search by filename, push/pull, test-auth) and git file history (list versions, view version, diff, restore + verify restored content) - `internal/test/githistorytest`, 8 cases, wired via `job.RunGitHistoryTest`/`POST /api/testdata/githistorytest`/admin button. Seeds a versioned file (2 commits, for diff/view/restore) and a file added-then-deleted (for latest-changes pagination ordering) in `test/git-history-tests`. The collection-filter case checks inclusion under the shared `test` collection and exclusion under a made-up collection name - since sample files live under `docs/test/` for the "Clean Test Data" button, there's no second real collection left to test exclusion against. The remote push/pull/test-auth case points the git remote at a throwaway local bare repo (`file://`, no network) and always restores whatever remote was configured before it ran; since `KNOV_GIT_REMOTE_BRANCH` isn't a live-editable setting, it also temporarily adds a local ref for the configured branch name pointing at HEAD when the repo's actual branch differs (e.g. default `master` vs. the configured default `main`), removing it afterward.

  Verified live: both suites pass independently (6/6, 8/8), `run-all` shows 48/48 across all four suites, and "Clean Test Data" removes every suite's sample files in one go.

  **fix (found while building step 5, unrelated to it):** `search-deleted-file-by-title`/`-content` started failing after a later product change (`git.IndexDeletedFiles`, the "index for deleted files" perf work) moved deleted-file search off a live commit-log walk onto a persisted index that's otherwise only populated incrementally by the cronjob. `resetAndSeed` already had this exact problem solved for full-content search (`search.IndexAllFiles()` synchronously after seeding) but the deleted-file index had no equivalent - fixed by capturing the commit hash right after the seed commit and calling `git.GetDeletedFilesSinceCommit` + `git.IndexDeletedFiles` synchronously after deleting delta, same reasoning as the existing `IndexAllFiles()` call. `run-all` is back to fully green (see step 4's verification below).
- [x] 3. chat - add/delete/get message (global + file-scoped), move, bulk move/delete, pagination - `internal/test/chattest`, 10 cases, wired via `job.RunChatTest`/`POST /api/testdata/chattest`/admin button. Covers add (global + file-scoped)/delete/get-by-id, `GetPage` pagination (limit truncation at `chat.PageSize`=50 + offset page), single move (append mode via `contentStorage`, new-file mode via a local `formatForEditorReplica`), bulk move (new-file mode), bulk delete, and the file-rename/file-delete cascades (`chat.MoveFilePath`, `chat.DeleteForFile`). `handleAPIMoveChatMessage`/`handleAPIBulkMoveChatMessages`/`handleAPIBulkDeleteChatMessages`/`formatForEditor` are all unexported in `internal/server`, so cases replicate their exact sequence of calls instead of calling the handlers - same approach as editorstest's bulk-metadata-patch case. editorstest's existing `caseBulkChatMoveDelete` (append-mode bulk move + bulk delete) was left as-is rather than migrated, since it's a reasonable smoke test in the editors context and chattest covers the same operations more thoroughly plus the new-file mode. Global (unscoped) chat messages aren't tied to any file path, so they can't be wiped via the `docs/test/` folder reset like every other suite's sample data - cases that add global messages delete them again themselves (`defer chat.Delete`), and cases using fixed file-scoped paths clean those via `chat.DeleteForFile` both at suite start and via `defer`, so a run is self-cleaning even if an earlier run left messages behind.

  Verified live: chattest passes independently (10/10) and after "Clean Test Data", `run-all` shows 56/58 across all five suites (the 2 failures were search's `search-deleted-file-by-*` cases, unrelated to this suite - see the fix noted under step 2).
- [x] 4. dashboard & kanban - dashboard (render each widget type, CRUD, import/export, rename); kanban (board load, filter, column order persists, card move) - split into two suites, same reasoning as step 2 (distinct concerns/subpackages):
  - dashboard - `internal/test/dashboardtest`, 9 cases, wired via `job.RunDashboardTest`/`POST /api/testdata/dashboardtest`/admin button. Covers create/get-all/update/rename/delete, export/import round-trip, and each widget type's underlying data resolution (filter widget via `filter.FilterFilesWithConfig`, fileContent via `files.GetFileContent`, tags/collections/folders via their `*CountFromCache` calls). `render.RenderWidget` (the actual render dispatch) lives in `internal/server/render`, which can't be imported here - same cycle noted for search's format rendering - so cases assert on the data each widget type resolves rather than on rendered HTML. Dashboards live in `configStorage` keyed by id, not under `docs/test/`, so (like chattest's global messages) fixed dashboard names are deleted at suite start via their derived id rather than relying on the folder wipe. The export/import case needed a distinct name for the imported copy - `dashboard.Create` derives id from name, so importing under the same name as the still-existing original collides, mirroring the real import form's "optional name override" requirement.
  - kanban - `internal/test/kanbantest`, 10 cases, wired via `job.RunKanbanTest`/`POST /api/testdata/kanbantest`/admin button. Covers board load/column bucketing, search-query narrowing, `SortCreatedAt`/`SortAlphabetical` (seeded so the two orders actually disagree, proving each is applied), card move (+ verifying the kanban event log), column-order persistence (`SaveOrder`/`GetOrder` applied on top of the baseline sort), the pure `ApplyOrder` function, `TagsForCollection`/`FilesForCollection`, `Excerpt`, and the pure tag helpers (`StatusFromTags`/`TagFromList`/`TagNotifyMsg`). `kanban.MoveCard` saves via `files.MetaDataSaveRaw`, which skips the cache refresh `MetaDataSave` normally does - cases call `files.InvalidateFileListCache()` afterward so `BuildBoard` sees the move immediately, the kanban analog of searchtest's synchronous reindex workaround. Sample cards' `CreatedAt` is pinned via a `MetaDataSave` (for title/collection/etc.) followed by a `MetaDataGet`+`MetaDataSaveRaw` round-trip, since `MetaDataSave` always stamps `LastEdited`/`CreatedAt` from the save call itself. The move-case's sample file also needed its kanban status tag stripped explicitly via `MetaDataSaveRaw` at seed time - `MetaDataSave` only overwrites `Tags` when the new value is non-empty (empty means "unspecified", not "clear it"), so a stale status tag from a previous run would otherwise survive the reseed. Card order/collection state (`kanban-order/test`) is config-store backed like dashboards, not touched by wiping `docs/test/`, so it's explicitly reset at suite start and via `defer` in the order-persistence case. The one genuinely untestable piece is native HTML5 drag-and-drop itself (browser-only JS) - per this doc's note below, the suite covers the API/state it drives (`SaveOrder`/`BuildBoard`) instead.

  Verified live: both suites pass independently (9/9, 10/10) and after "Clean Test Data", `run-all` shows 75/77 across all seven suites (the 2 failures were search's `search-deleted-file-by-*` cases, unrelated to this step - fixed afterward, see step 2's addendum; `run-all` is 77/77 as of that fix).
- [ ] 5. browse & info slideout - browse/icons (`/browse/files`, `/browse/media`, `/browse/{metadata}[/{value}]`, file tree, folder contents, autocomplete); metadata (get/set all fields, inline-display/inline-edit); TOC (header extraction); references (add/remove/list); connections (parents/ancestors/kids/grandchildren/related/used-links/links-to-here, conflict banner+diff)
- [ ] 6. jobs, media, admin - jobs (metadata-rebuild, search-index, media-cleanup, cache-invalidate, manual trigger, status/history) - assert on filesystem/DB state, not just success; media (upload, list, preview, rename, delete, orphaned-cleanup, stats); admin actions (cache invalidate, git push/pull/test-auth, data path change); export/import (markdown, zip, metadata export, dashboard/settings export->import round-trip)
- [ ] 7. settings, notifications, logs - notifications (flash consumed once, persistent list, delete one, clear all); settings/themes/config (bulk+individual settings, theme list/switch/settings, config repo url/data path/favicon/languages); logs (in-memory list, file pagination/chunking, download)

note: browser-only interactions (kanban drag-and-drop, toastui toolbar) can't be verified by an in-app runtime suite the same way - either accept coverage of the underlying API/state instead, or handle those specific cases via the `testkit` chromedp path separately.

**htmx/JS call inventory (builtin theme, ~45 endpoints - reference for scoping suite cases above, not a separate step)**
- admin.gohtml: metadata rebuild, cronjob trigger, cache invalidate, restart, config repository get/post, git pull/push/test-auth, config datapath get/post, media stats, media cleanup-orphaned, config import, metadata editors?format=options (x4), testdata setup/clean/filtertest(+testdata)
- base.gohtml: search?format=list, filters/add-criteria, filters (execute), metadata/references (post)
- files/browse: files/bulk (delete), files/browse?metadata=&value=, files/list(?actions=true), files/tree?actions=true, files/folder?path=, files/rename/*, files/move-folder/*, files/delete/*, files/versions/* (+?output=full), files/versions/restore/*, files/versions/diff/*
- editors: editor?filepath=&section=, editor?editor=&prefillpath=, editor?filepath= (filter editor), editor/tableeditor?filepath=&tableIndex=
- metadata/slideout: metadata/tags|collections|folders|editors, metadata/createdat|lastedited|collection|folders?filepath=, metadata/inline-display?field=, metadata/references?filepath=, metadata/rebuild/*, metadata?filepath=media/...
- connections: links/ancestors|kids|grandchildren|used|media|linkstohere|related?filepath=, links/conflicts/banner + conflicts/of-banner
- search: search?format=cards, search?format= (dynamic), git/latestchanges?count=50[&q=][&collection=]
- dashboards: dashboards/form[?id=], dashboards/widget/{id}, dashboards + dashboards/* (sidebar list)
- kanban: kanban/{collection} (board load/refresh), kanban/card/move (drag-drop)
- chat: chat/bulk-form?mode=, chat/messages/bulk/move, chat/messages/bulk (delete)
- settings/themes: settings/table|editor|file-types|general|media, themes/settings
- media: media/list?mode=compact&filter=
- notifications: notifications (list)
- filters: filters/criteria-row?row_index=, filters/* (saved list, data-url driven)
- unmapped/needs tracing before writing tests: base.gohtml:360 `hx-post=""` and :374 `hx-delete=""` (target injected at runtime, not static)

# docs

small, precise and concise, high level overview, no examples that are prone to change, just a few bullet points, as few subheaders as possible (i think it becomes more unreadable if its too segmented)
