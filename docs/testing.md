# Testing

Go native only. Two tiers, depending on whether real browser/JS behaviour is under test.

**Tier 1 - `net/http/httptest`**
- Boots the real chi router (`server.NewRouter()`) against a temp data dir and a freshly `git init`'d repo, via `testkit.NewApp(t)` in `internal/testkit/testkit.go`
- Runs the same init sequence as `main.go`: content storage, all sqlite-backed storages, theme manager, settings
- Requests go through real HTTP against `httptest.Server`; assertions check status code and parsed HTML fragments, not raw string snapshots
- Covers everything that doesn't require a real browser: filters, editors, search, git history, chat, dashboards, kanban API, metadata/links, jobs, media, admin actions, settings, notifications, logs

**Tier 2 - `chromedp`**
- Pure Go, drives a real headless Chrome over the DevTools protocol - no Node/npm involved
- Reserved for behaviour that only exists in the browser: drag-and-drop (kanban), the toastui rich editor, confirming an htmx swap actually lands in the DOM
- Also uses `testkit.NewApp(t)`, so there's one bootstrap for both tiers
- Static/theme JS and CSS assets are not wired into testkit yet, so tests needing real client-side JS (not just DOM/navigation checks) need that fixed first

**Package layout**
- `internal/testkit` holds the bootstrap itself (not a `_test.go` file, so any package's tests can import it) - it only depends on `server.NewRouter()` and other packages' exported `Init` functions, no unexported access needed
- Tests that use testkit must live in an external test package (`package server_test`, not `package server`) - testkit importing `server` back would otherwise be a real import cycle for internal test files
- Go test files live next to the code they cover, one `_test.go` per `api_*.go` in `internal/server` is the target structure
- `internal/test` holds the in-app "seed test data" feature (creates sample files/git history for manual QA) - unrelated to the Go test suite, just named similarly

**Singletons**
- App state (content storage, metadata/search/chat/cache/notification storage) lives in package-level singletons, same as in production
- Only one test app can be "live" per process at a time - tests using testkit must not call `t.Parallel()`

**Scope**
- The full test-case checklist and the htmx/JS call inventory backing it live in `docs/temp_todo.md` under `# testing`
