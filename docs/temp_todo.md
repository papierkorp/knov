# temp todo

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

**per ai**
- not important
  - create a system for themes (another repoistory with themes)
    - .e.g. create a table/dict with all top level folders - than check if there is a theme.json
  - deployment
    - make docker build viable
      - for usage
      - for devs
  - update/change the fontpreview solution (pdfexport) - i dont like it (only setting to touch the DOM structure around the `<select>`)
- features
  - backup solution — see `# backup-solution` below
  - todo editor with markdown on top/bottom + header
  - update hide paths (filevisibility) to only hide from certain features (e.g. hide in tree but show in browse)
  - add go tests?
  - new kanban feature: create folders based on the kanban status and move the files based on the kanban status => what would be the single source of truth - i think the metadata and if a file in such a folder doesnt have metadata => add it / change it
  - gitlab pipeline to get a release
  - in the theme let me also arrange the info slideout components
- fixes
- chore
  - move copy-code.js to the app
  - storage/config/theme/xxx.json => do we need to add escapes in this json file? i dont like it
  - move translation and changelog to a tools/ package just like templatedocs

# every other time

- take a look at all routes if we use writeResponse everywhere neccessary and if we can update the functions where we only use json to htmx as well
- take a look at the whole codebase into all javascript snippets/scripts with the goal of reducing javascript in favor of more htmx - im also fine with refactoring to make this to work since i think we already use a lot of javascript which could be resolved using htmx
- pass over css files (components.css/panels.css/layout.css) for dead selectors, confirm remaining ones follow the id-selector convention
- check the whole codebase for hardcoded colors and replace theme with the vars provided by the defaults.css file

# backup-solution

Goal: reliable, easy-recovery backups for StoragePath (metadata/cache/chat/kanban/notifications/config/search) — DataPath is already covered by git. List backups on a new page, restore any of them, keep the set trimmed automatically. contentStorage skipped for now (git already covers it).

Decided: interface-per-storage, not pure filesystem-level. New `internal/backup` package holds the type-aware helpers (`BackupSQLite`/`RestoreSQLite` via `VACUUM INTO`, `BackupFile`/`RestoreFile` for json/yaml) — same shape as `dbmigration`. Each storage interface gets thin `Backup(destDir) error` / `Restore(srcDir) error` methods (one-line bodies delegating to the shared helper), including a no-op on `kanbanStorage_noop.go`.

## prerequisites
- `configStorage_json.go`, `metadataStorage_json.go`, `cacheStorage_json.go` write via plain `os.WriteFile` (truncate+write), not atomic — switch to temp-file-then-rename before backup can safely copy them live (a backup mid-write could otherwise read a torn file)

## design decisions
- sqlite `Backup()` takes the storage's existing `sync.RWMutex` `RLock` around `VACUUM INTO` for a consistent per-storage snapshot — no new locking primitive needed
- no cross-storage atomicity — storages are snapshotted sequentially, each under its own lock, not one transaction. fine for single-user use, but say so on the `/system/backup` page rather than implying a single point-in-time snapshot
- restore does not hot-swap live `*sql.DB`/in-memory state — write restored files to disk, then reuse the existing restart pattern (`handleAPIRestartApp` / `os.Exit(0)`), same as the "data path changed, needs restart" flow
- partial backup failure (storage N of M fails mid-run) aborts and discards the whole backup rather than listing an incomplete one — a broken backup that looks valid is worse than no backup

## todo
- factor out the duplicated "serve a file for download with headers + error handling" boilerplate between `handleAPIExportSettings` and `handleAPIExportMetadata` into one small helper — that's the actual duplication, not the underlying concept
- a small `BackupTarget` interface (`Write`, `List`, `Read`, `Delete`) with a local-filesystem implementation first, S3/NFS added later as new files without touching callers
- `/system/backup` fits alongside the existing `/system/logs`, `/system/jobs` pages. Restore is destructive, so run it through `internal/job` as a manual job (gets logged into `JobRun` history like `gitPushJob` etc.) and auto-snapshot current state immediately before overwriting it — free "undo" for a bad restore, same safety-net idea as the git auto-push
- reuse the GFS (grandfather-father-son) idea already sketched — a different shape than `logging_rotate.go`'s count-based shifting (which just cares about N most recent files). Needs date-bucketing logic instead: keep everything ≤7 days, thin to one/day up to 30 days, one/month up to 365 days. Worth a small `backup/rotate.go` rather than reusing the log rotator directly, since the algorithms don't overlap much

# Async job system — plan

Goal: fix slow/blocking delete (single file cache race + synchronous
folder/bulk delete), by building a reusable async job runner with
persisted state, crash recovery, and htmx status polling.

## Step 1 — `jobStorage` package (persistence) — DONE
- New `internal/jobStorage` following the existing per-domain pattern
  (`internal/notificationStorage` as template): `_interface.go` +
  `_sqlite.go`, `Init(storagePath)`, own migration in `internal/dbmigration`.
- Table: id, job type, args (JSON blob — e.g. target file list), status
  (running/done/error/interrupted), started_at, finished_at, error message.
- Minimal ops needed: `Create`, `UpdateStatus`, `Get(id)`, `ListRunning()`.
- Wired into `main.go` init sequence alongside the other storages.

## Step 2 — `Job` interface changes (`internal/job`) — DONE
- Added `Resumable` as an optional interface (`Resumable() bool`), same
  shape as the existing `Outputter`/`Messenger` optional interfaces —
  checked via type assertion, not required on every `Job`.
- `Run()` unchanged; business logic doesn't change.

## Step 3 — Async runner (`internal/job`) — DONE
- Named `StartAsync` (not `RunAsync` — that name is already taken by the
  existing no-arg "run all jobs now" admin-button trigger in
  `scheduler.go`/`api_cronjob.go`; keeping both avoids a collision/rename
  of an existing public API).
- `StartAsync(mu *sync.Mutex, job Job, args string) (id string, err error)`:
  - `mu.TryLock()` synchronously so the caller gets `ErrAlreadyRunning`
    immediately, same guarantee the old `RunAsync()` has
  - persists job record via `jobStorage.Create` (status=running) *before*
    spawning the goroutine, so a poll right after `StartAsync` returns
    always finds the record
  - goroutine runs the shared `runLocked(job)` (factored out of `execute`,
    holds history/panic-recovery logic) and updates `jobStorage` on finish
- Existing synchronous `execute()`/`Run()` path untouched for anything that
  doesn't need async (don't force-migrate everything at once).
- Not yet used by any caller — next step (4) needs a job-type registry
  (mirroring `externalsuite.go`'s `suiteRunners` map) to reconstruct a Job
  from persisted `(type, args)` on startup recovery, then step 7 wires
  actual delete handlers to call it.

## Step 4 — Startup recovery hook — DONE
- `job.RecoverInterrupted()`, called from `main.go` right after
  `jobStorage.Init` (before `job.Start()`). New `internal/job/asyncdelete.go`
  holds a `resumers` map (`Name() -> func(args) (Job, *sync.Mutex, error)`,
  mirroring `externalsuite.go`'s `suiteRunners`) for `delete-folder` and
  `bulk-delete-files`.
- Resumable type found → `StartAsync`'d fresh (new id) with the persisted
  args; the stale `running` record is marked `interrupted` (reason "resumed
  as a new job after restart") rather than reused, to avoid two goroutines
  racing to update the same jobStorage row.
- Non-resumable/unrecognized type → marked `interrupted` + a pending
  notification via `notificationStorage.Add(..., true)` directly (same
  precedent as `git.go`'s conflict notifications - bypasses `notify.SetFlash`
  since there's no in-flight request to attach it to).
- Folder-delete resumability: `StartDeleteFolder` resolves the folder to a
  file list via the new `files.ListFilesInFolder` *before* persisting the
  job, and that resolved list (not the folder path) is what's replayed on
  resume - see Step 7.

## Step 5 — Status endpoint (server package) — DONE
- `GET /api/jobs/{id}` in new `internal/server/api_jobs.go`: reads
  `jobStorage.Get(id)` directly (server already imports other `*Storage`
  packages directly, e.g. `notificationStorage`, `kanbanStorage`), renders
  via `render.RenderJobStatus`. Unknown id → 404 `writeAPIError`.
- On a terminal state it also fires the toast (`notify.SetHeader`) or queues
  the flash + `HX-Redirect` (`notify.SetFlash`, bulk-delete only - see Step
  6) - so the *first* poll response after completion is what the user sees,
  not a separate mechanism.
- RESTful, swagger comment (`swag init` re-run, docs regenerated), `writeResponse`,
  `translation.SprintfForRequest` per project conventions.

## Step 6 — htmx polling partial (render + theme templates) — DONE
- `render.RenderJobStatus` (bare self-polling `<span id="job-status-{id}">`,
  `hx-get hx-trigger="every 1s" hx-swap="outerHTML"` while running; empty
  span when done; inline `<i class="fa fa-triangle-exclamation">` + message
  on error/interrupted) + `render.RenderJobStatusListItem` (same, wrapped in
  `<li>` for the one-time swap of a browse-tree row, so the parent `<ul>`'s
  content model stays valid across every later self-swap of just the span).
- Terminal states carry no `hx-trigger` → polling stops itself, no extra
  bookkeeping needed.
- `.job-status-pending` / `.job-status-failed` added to
  `themes/builtin/css/components.css` (global reusable classes, next to the
  existing `.status-ok/.status-error/...`), `var(--danger)` only, no
  hardcoded colors. `themes/example` is a stub reference theme (every
  relevant page is `{{ define "content" }}{{ end }}`) - nothing to update
  there.

## Step 7 — Wire up folder/bulk delete — DONE
- `handleAPIDeleteFolder` / `handleAPIDeleteFilesBulk` now call
  `job.StartDeleteFolder` / `job.StartBulkDeleteFiles` and return the
  polling fragment immediately instead of blocking.
- `deleteFolderJob` reshaped to take a pre-resolved `fullPaths []string`
  (like `bulkDeleteFilesJob` already did) instead of a folder path it
  re-walked internally - `files.DeleteFolder` (which did the walk *and* the
  `os.RemoveAll` in one call, making the walk-then-delete non-resumable) was
  replaced by `files.ListFilesInFolder` (walk only) + `files.BulkDeleteFiles`
  (already resumable/idempotent-ish - re-deleting an already-gone path just
  logs a warning and continues) + a final `os.RemoveAll` to clean up the
  now-empty folder tree.
- Old synchronous `job.RunDeleteFolder`/`RunBulkDeleteFiles` removed (no
  other callers - grepped test suites and server package first).
- Granular git commits: unchanged from before this pass -
  `git.CommitDeletedFiles` still fires once per job run, not per file. Left
  as a Step 9 follow-up rather than bundled in here (independent of the
  async plumbing itself).
- Frontend: builtin theme's bulk-delete button changed from `hx-swap="none"`
  to `hx-target="this" hx-swap="outerHTML"` so it can show the polling
  fragment; on completion the job-status endpoint sets `HX-Redirect` (same
  target as the old synchronous handler) since that page's file list still
  needs a full reload to drop the deleted files - only the *blocking*
  behavior needed fixing there, not the redirect-on-success UX.

## Step 8 — Fix single-file delete — DONE
- Root cause confirmed: `RefreshCaches()` → `InvalidateFileListCache()`
  synchronously nils the in-memory memo *and* deletes the on-disk cache
  entry, then rebuilds in the background - any `GetAllFilesCached()` call
  in that window (e.g. the `/browse` tree fragment right after a delete)
  falls through to a full synchronous disk walk.
- Fix: `files.RefreshCachesAfterDelete(deletedPath)` (new, used only by
  `removeFileAndMetadata`) removes just that one path from the memo + cached
  JSON in place (matched via `pathutils.ToRelative` on both sides, so it's
  robust to prefix-format differences) instead of wiping the whole list,
  then rebuilds the other aggregate caches in the background same as
  `RefreshCaches`. Falls back to a full `InvalidateFileListCache` if the
  memo isn't currently populated (nothing to remove from).
- `RefreshCaches` itself is untouched - still used by edits/renames/moves,
  which don't have a single well-known deleted path to splice out.

## Step 9 — Follow-up candidates (not now)
- Route other slow, HTTP-triggered ops through `StartAsync` once the base
  system is proven on delete. Candidates (priority order), all currently
  blocking their request handler:
  - `job.RunMoveFolder` (`api_files.go:919`) - walks a folder and rewrites
    links for every file inside, same shape `deleteFolderJob` had pre-async.
    Best first pick - structurally closest to what's already built.
  - `job.RunBulkUpdateMetadata` (`api_metadata.go:114`) - patches every file
    in a matched set, same shape as `bulkDeleteFilesJob`.
  - `job.RunFullRebuild` (`api_metadata.go:225`) - full-vault metadata cache
    rebuild.
  - `job.RunRepairBrokenLinks` (`api_metadata.go:343`) and
    `job.RunMediaCleanup` (`api_media.go:399`) - both scan/mutate an
    unbounded number of files.
  - `job.RunGitPush`/`RunGitPull` (`api_git.go:119,139`) - network calls,
    can hang on a slow remote regardless of file count.
  - Not worth it: test-suite runners (`RunFilterTest` etc.) and
    `RunCacheInvalidate` - admin/dev-facing, synchronous pass/fail is fine.
  - None of these are safely resumable like delete is (no clean "replay a
    move/patch after a crash" without checking what already landed) - set
    `Resumable() bool { return false }` and get non-blocking + polling +
    interrupted-notification only, not crash-safe resume.
- Before adding more job types, generalize two spots that are currently
  delete-specific despite `StartAsync`/`jobStorage`/`RenderJobStatus` being
  generic:
  - `handleAPIGetJobStatus`'s completion handling (`api_jobs.go`) hardcodes
    a `switch` on `rec.Type` for the toast text and the bulk-delete-files
    redirect special-case - needs a per-job-type success-message lookup (or
    lean on `Messenger`) instead of growing that switch per job.
  - `asyncdelete.go`'s `resumers` map is structurally reusable but named/
    scoped as delete-only - either rename/generalize the file or add a
    sibling file per job group that contributes to a shared registry.
- More granular git commits during `deleteFolderJob`/`bulkDeleteFilesJob`
  (currently one commit at the end, same as before this pass) so an
  interrupted run leaves a cleaner partially-committed state.
- `RecoverInterrupted`'s resumed jobs get a fresh job id - if the UI is ever
  mid-poll on the old id when a restart happens, that poll will 404 once
  and the user has to reload to pick up the new id. Fine for now (a crash
  mid-delete is already a rare/degraded path), but worth a "poll redirects
  to new id" affordance if it comes up in practice.

---
Key files found during investigation:
- `internal/server/api_files.go:961` handleAPIDeleteFile
- `internal/server/api_files.go:1000` delete-folder route
- `internal/server/api_files.go:1045` bulk delete route
- `internal/files/metadata_cache.go:152-172` cache invalidation/rebuild race
- `internal/job/scheduler.go:48-88,255-280` execute(), RunDeleteFolder etc.
- `internal/job/history.go` in-memory job history (ring buffer)
- `internal/job/manualjob.go:324,367` synchronous git commit inside jobs
- `internal/git/git.go:372-410` CommitDeletedFiles, `:482-521` CommitDeletedFile (async)
- `internal/notificationStorage/notificationStorage_interface.go` pending-flash pattern to reuse for "job interrupted" messages
- `internal/dbmigration/dbmigration.go` migration pattern to follow for new jobStorage table


# ai prompts

## docs

small, precise and concise, high level overview, no examples that are prone to change, just a few bullet points, as few subheaders as possible (i think it becomes more unreadable if its too segmented)

## review

Role: Act as a Staff-Level Software Engineer conducting a code review with a high bar for quality and maintainability.

Task: Review the current git diff. You are strictly prohibited from rewriting the code or providing "fixed" code snippets. You are only permitted to give your professional opinion on the changes.

Constraints:
- Do not output any code. Do not suggest code blocks, patches, or refactored versions of the provided diff.
- Verdict: If the changes are completely safe, logically sound, and meet standard best practices, explicitly state: "VERDICT: APPROVED" in your response.
- Problems: If you find any issues, do not fix them. Instead, explain why they are problematic, the potential impact (e.g., runtime error, security hole, performance bottleneck, unreadability), and optionally, the strategy to fix them (without writing the actual code).

Areas to scrutinize (your opinion must cover these):
- Correctness: Are there off-by-one errors, incorrect variable reassignments, or logical flaws?
- Edge Cases: Will this fail on empty arrays, null values, or extreme inputs?
- Security: Does this introduce injection risks, exposed secrets, or unsafe deserialization?
- Performance: Are there O(n²) loops hiding in the changes, or unnecessary database queries?
- Maintainability: Is the naming clear? Is it adding accidental complexity or tight coupling?
- Side Effects: Are there changes to global state, environment variables, or external APIs that weren't considered?
- Architecture: are the changes in line with the rest of the codebase?

Also give your opinion about the changes
