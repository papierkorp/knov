# Testing

In-app runtime test suites - not `go test`. Knov ships as a single binary with no go toolchain on the target machine, so tests need to be runnable against a real running instance, from an admin button or an API call. The existing filter test (`internal/test/testfilter.go`, wired to the admin page and `POST /api/testdata/filtertest`) is the model every other suite follows.

**Suite interface**
- `internal/test` defines the shared shape every suite returns: `CaseResult` (name, free-form `Expected`/`Actual` strings, error, success, `Detail any` for suite-specific extras) and `SuiteResult` (suite name, totals, pass/fail, list of `CaseResult`), plus a `Suite` interface (`Name() string`, `Run() (*SuiteResult, error)`)
- `Expected`/`Actual` are plain strings rather than typed values, since suites compare very different things (a list of matching files, a single pass/fail, rendered content) - each suite formats its own comparison text

**Package layout**
- One subpackage per test group under `internal/test/`, e.g. `internal/test/filtertest`, `internal/test/editorstest` - each seeds real files/metadata via the internal packages directly (no HTTP round-trip) and implements `Suite`
- Subpackages are always suffixed `test` (`filtertest`, not `filter`) - a subpackage named `filter` would collide with `knov/internal/filter` in every file that needs both (job wrapper, API handler), forcing an import alias everywhere; the suffix avoids that
- `internal/test/registry.go` holds the list of suites and `RunAllTests()`, which runs them in order and aggregates - adding a suite later means adding its subpackage and one line here

**Wiring (same shape for every suite, including `RunAllTests()` itself)**
- A `job.Job` wrapper in `internal/job` (mutex-guarded via `execute()`, recorded in job history, visible at `/system/jobs`)
- An HTTP handler in `internal/server`, swagger-annotated, at `POST /api/testdata/<group>test`
- A button on the admin page

**Where `internal/testkit` fits**
- `internal/testkit` (`httptest` + `chromedp`) is not the primary vehicle for suites - it stays around for the rare case a suite genuinely needs a real HTTP/router pass, and for the handful of things an in-app suite structurally can't verify: real browser/JS interaction like kanban drag-and-drop or the toastui editor toolbar
- For those, cover the underlying API/state through a normal suite, and only reach for `testkit`'s chromedp path if the interaction itself needs checking

**Scope**
- The suite build order and the htmx/JS call inventory backing it live in `docs/temp_todo.md` under `# testing`
