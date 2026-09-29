# Testing

Three kinds of tests live in this repo:

- **In-app runtime suites** (`internal/test/<x>test`, run via `knov --start-tests`) - for anything needing live data/storage or real installed theme files that a sandboxed `go test` can't fake. Knov ships as a single binary with no go toolchain on the target machine, so these run against a real built binary instead.
- **Browser-driven cases** (`internal/testkit`, chromedp) - a handful of cases living inside the suites above, for real browser/JS interactions those suites structurally can't verify by calling Go functions directly.
- **Plain `go test` files** colocated in the package they cover - for pure logic or anything backed only by a package-level store (e.g. `configStorage`) that a `TestMain` can point at a temp dir.

## In-app runtime suites
- One subpackage per group under `internal/test/`, suffixed `test` (`filtertest`, not `filter`) to avoid import collisions with the package it exercises
- Standard layout per subpackage: `<group>test.go` (just the `Suite` type), `sampledata.go` (seed/wipe/reseed), `testcases.go` (the cases, split into `testcases_<category>.go` once there's enough to warrant it)
- Suites self-register via `test.Register(Suite{})` in their own `init()`; adding a suite means a new subpackage plus a blank import in main.go
- Each suite's sample files live under `docs/test/` so the admin "Clean Test Data" button clears them all at once
- `knov --start-tests` (optionally with a suite name) is the only entry point; it runs headless against empty isolated data/storage dirs and an isolated log dir (only the settings are copied over, the git remote is disabled) so live data and the real remote are never touched and no second copy of the data is needed
- Suite build order and coverage gaps are tracked in `docs/temp_todo.md`

## Browser-driven cases
- For interactions a suite structurally can't reach any other way (native drag-and-drop, a vendored JS library's undo/redo, toolbar wiring) - cover everything reachable through the normal suite first, and only reach for this when the interaction itself needs checking
- Renders the real fragment/vendored JS-CSS behind an `httptest.Server`, drives it with chromedp, and waits on concrete conditions rather than a fixed sleep
- Skips (not fails) a case when no local Chrome/Chromium is found; chromedp still ends up statically linked into the shipped binary, since there's no separate test build
- Currently hardcodes the builtin theme's DOM/JS, so these are builtin-only integration tests for now - revisit once a second theme exists
- No separate command or flag - each case is a regular entry in its owning suite's case list, so `knov --start-tests` (or `--start-tests <suite>`) starts it the same way as every other case

## Plain `go test` packages
- Ordinary `_test.go` files, run with `go test ./...`, no `--start-tests` involved

When adding a new case: a real browser/JS interaction belongs in a suite's `testcases_browser.go`; pure logic or a temp-dir-backed package belongs in a colocated `_test.go`; anything needing live data or real installed files belongs in a new `internal/test/<x>test` suite.
