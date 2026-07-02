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
- add chat to a single file?
- version change to a generated file?
- codemirror copy + paste with y+p does not work properly (e.g. i have something - in the clipboard and it doesnt paste and i need to use ctlr+v in edit mode)
- 
- deployment
  - make docker build viable
    - for usage
    - for devs
- backup solution

# performance updates

**read**
- use Query() instead of a loop through files.GetAllFiles()
- use Query in filter.go
- Refactor filter.go to use query

**write**
- add a SetBatch for rebuild metadata
- But if you ever wanted to make it faster the right approach would be to build a one-time index of deleted files at startup/cronjob time and search that instead of walking commits live. (git history search)

**concurrent**
- make rebuild metadata concurrent e.g. each letter?

dont give me any code yet - what options do i ave to make the rebuild metadata concurrent?
at the moment im running through ALL files mutliple times (e.g. with MetaDataPurgeStale, MetaDataPurgeDuplicates, linksrebuild..)May 13Claude responded: Looking at the current flow, you have roughly:Weighed concurrent metadata rebuild strategies to eliminate redundant file iterationsWeighed concurrent metadata rebuild strategies to eliminate redundant file iterationsLooking at the current flow, you have roughly:

Walk all files → build/update metadata (sequential)
Walk all metadata → purge stale (sequential)
Walk all metadata → purge duplicates (sequential)
Walk all metadata → rebuild links (sequential)

Options:
A — Single pass, pipeline everything
One walk over all files, do metadata update + link extraction in the same iteration. Then one walk over metadataStorage for purge stale + duplicates combined. Cuts the number of full scans from ~4 to 2. No concurrency needed, just smarter ordering.
B — Worker pool per file
Keep the multi-pass structure but process each file concurrently using a sync.WaitGroup + buffered channel as a worker pool. Most useful for the rebuild step since it does I/O (read file → extract links → write metadata). Stale/duplicate purges are already just map operations so they're fast.
C — Fan-out with errgroup
Use golang.org/x/sync/errgroup with a semaphore to bound concurrency. Cleaner error handling than raw goroutines. Same idea as B but more idiomatic.
D — Combine A + C
Single pass over files with a worker pool, doing metadata + link extraction per file concurrently, then one combined purge pass at the end. Best of both worlds.
My recommendation: D. The real bottleneck is the per-file I/O in the rebuild (reading file content for link/title extraction). Parallelizing that with a bounded worker pool (e.g. runtime.NumCPU() workers) gives the biggest win. The purge passes are already fast (just map lookups) so combining them into one pass is enough there — no need to parallelize.
The main thing to watch out for: metadataStorage writes need a mutex or you need to collect results and write sequentially after the workers finish.
Want to go with D?

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
- `internal/test/registry.go` holds `var suites = []Suite{filtertest.Suite{}, editorstest.Suite{}, ...}` and `RunAllTests() (*SuiteResult, error)` loops over it and aggregates - adding a group later is one line here plus its subpackage.
- per-suite wiring, same shape as the existing filter test: a `job.Job` wrapper in `internal/job` (mutex + history, `execute()`), an HTTP handler in `internal/server` (`POST /api/testdata/<group>test`, swagger-annotated), and an admin button. `RunAllTests()` gets the same treatment (its own job wrapper + `POST /api/testdata/run-all` + "run all tests" button).
- `internal/testkit` (httptest+chromedp harness from step 0) is not the primary vehicle for any of this - keep it around only for the rare case a group genuinely needs a real HTTP/router pass or actual browser JS check.

**0. build the architecture + rewrite filtertest as suite #1 (do first, step by step)**
- [ ] add `Suite`/`SuiteResult`/`CaseResult` types to `internal/test` (new file, e.g. `internal/test/suite.go`)
- [ ] create `internal/test/filtertest/` package; move `testfilter_testfiles.go` + `testfilter_testmetadata.go` into it unchanged (just package rename, fixtures stay as-is)
- [ ] rewrite `testfilter.go` into `internal/test/filtertest/filtertest.go`: `CaseResult`/`SuiteResult` instead of `FilterTestResult`/`FilterTestResults`, add a `Suite`-implementing type (`Name() string { return "filter" }`, `Run()` wraps the existing `testConfigs` table, one `CaseResult` per config), delete the old `internal/test/testfilter.go`
- [ ] update every caller of the old types: `internal/job/cronjob.go` (`filterTestJob`, `RunFilterTest`), `internal/job/scheduler.go` wrapper, `internal/server/api_testdata.go` (`handleAPIFilterTest`, `handleAPIFilterTestMetadata`) + swagger annotations
- [ ] update `render.RenderFilterTestResults` (or replace with a generic `SuiteResult` table renderer reusable by every future suite) so the admin page still renders correctly
- [ ] add `internal/test/registry.go`: `suites []Suite` + `RunAllTests() (*SuiteResult, error)`
- [ ] wire `RunAllTests()` through its own job wrapper + `POST /api/testdata/run-all` + admin "run all tests" button
- [ ] manual check: admin page "run filter tests" button and new "run all tests" button both work against a real running dev instance, endpoint/button locations unchanged (no user-facing change)

**suite build order** (after step 0; filter is done as part of step 0, so this starts at the next group)
- [ ] 1. editors - create+edit+save for toastui, textarea, codemirror, filter, list, todo, index, table; section-save, table-save, todo-toggle, convert-to-markdown; file rename/move (updates links elsewhere); bulk ops (bulk delete files, bulk metadata patch, bulk chat move/delete)
- [ ] 2. search & history - search (title-only, full-content, deleted-file/history, all response formats, empty query); git repo history (latest-changes pagination, filter by collection, search by filename, push/pull, test-auth); git file history (list versions, view version, diff, restore + verify restored content)
- [ ] 3. chat - add/delete/get message (global + file-scoped), move, bulk move/delete, pagination
- [ ] 4. dashboard & kanban - dashboard (render each widget type, CRUD, import/export, rename); kanban (board load, filter, column order persists, card move)
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
