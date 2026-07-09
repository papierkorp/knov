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
- A `job.Job` wrapper in `internal/job` (mutex-guarded via `execute()`, recorded in job history, visible at `/system/jobs`)
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
