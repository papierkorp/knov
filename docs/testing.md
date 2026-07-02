# Testing

In-app runtime test suites - not `go test`. Knov ships as a single binary with no go toolchain on the target machine, so tests need to be runnable against a real running instance, from an admin button or an API call. The filter suite (`internal/test/filtertest/`, wired to the admin page and `POST /api/testdata/filtertest`) is the model every other suite follows.

**Suite interface**
- `internal/test` defines the shared shape every suite returns: `CaseResult` (name, free-form `Expected`/`Actual` strings, error, success, `Detail any` for suite-specific extras) and `SuiteResult` (suite name, totals, pass/fail, list of `CaseResult`), plus a `Suite` interface (`Name() string`, `Run() (*SuiteResult, error)`)
- `Expected`/`Actual` are plain strings rather than typed values, since suites compare very different things (a list of matching files, a single pass/fail, rendered content) - each suite formats its own comparison text

**Package layout**
- One subpackage per test group under `internal/test/`, e.g. `internal/test/filtertest`, `internal/test/editorstest` - each seeds real files/metadata via the internal packages directly (no HTTP round-trip) and implements `Suite`
- Subpackages are always suffixed `test` (`filtertest`, not `filter`) - a subpackage named `filter` would collide with `knov/internal/filter` in every file that needs both (job wrapper, API handler), forcing an import alias everywhere; the suffix avoids that
- `internal/test/registry.go` holds `RunAllTests()`, which runs the registered suites in order and aggregates. Suites self-register via `test.Register(Suite{})` in their own `init()` (a `<group>test` package importing `internal/test` for the shared types rules out `internal/test` importing back to build the list directly) - adding a suite later means adding its subpackage plus that `init()` line

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
- Wipes and reseeds its own fixture folder at the start of every run, then runs one independent case per editor operation: create+edit+save for every editor type, section save, table save, todo-toggle, convert-to-markdown, file rename/move, and the bulk ops (delete, metadata patch, chat move/delete)
- Editor HTTP handlers mix request parsing with business logic inline, so there's usually no single function to call directly - cases instead call the same underlying functions the handler calls (content storage write + metadata save + link rebuild, the content handler's section/table save, todo state cycling, the dokuwiki converter, etc.), reproducing the handler's real sequence of calls without an HTTP round-trip
- Two bulk-op cases (metadata patch, chat move) can't reach their handler's actual logic because it's unexported in `internal/server` - those replicate the same behavior using the equivalent exported building blocks instead
