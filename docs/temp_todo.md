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
  - refactor template_data.go make it more general/modular/uniform/ easier for themecreators, e.g. only one general pass and not mutliple different ones => would it make a performance difference?
  - move copy-code.js to the app
  - storage/config/theme/xxx.json => do we need to add escapes in this json file? i dont like it

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

## Step 1 — `jobStorage` package (persistence)
- New `internal/jobStorage` following the existing per-domain pattern
  (`internal/notificationStorage` as template): `_interface.go` +
  `_sqlite.go`, `Init(storagePath)`, own migration in `internal/dbmigration`.
- Table: id, job type, args (JSON blob — e.g. target file list), status
  (running/done/error/interrupted), started_at, finished_at, error message.
- Minimal ops needed: `Create`, `UpdateStatus`, `Get(id)`, `ListRunning()`.

## Step 2 — `Job` interface changes (`internal/job`)
- Add resumability: e.g. `Resumable() bool` on `Job` (default false via a
  base/helper, explicit true for delete-type jobs).
- Keep `Run()` as is; business logic doesn't change.

## Step 3 — Async runner (`internal/job`)
- `RunAsync(job Job) (id string, err error)`:
  - persist job record via `jobStorage` (status=running) before starting
  - launch existing `execute()`/`Run()` in a goroutine
  - update `jobStorage` (+ existing in-memory history) on finish
- Existing synchronous `Run()` path stays for anything that doesn't need
  async (don't force-migrate everything at once).

## Step 4 — Startup recovery hook
- On app init, after `jobStorage.Init`, scan `ListRunning()`:
  - if `Resumable()` job type → re-invoke via `RunAsync` with persisted args
  - else → mark `interrupted`, push a message via existing
    `notificationStorage` pending-flash mechanism so the user sees it on
    next page load
- For folder-delete specifically: persist the *resolved file list* at job
  start, not the folder path — avoids deleting files added to that folder
  after a crash (re-walking would not be idempotent, a snapshot list is).

## Step 5 — Status endpoint (server package)
- `GET /api/jobs/{id}` — thin handler, reads `jobStorage`/history, renders
  fragment via `render` package (spinner while running, success/error
  when terminal). Unknown id → explicit error state, not "still running".
- RESTful, swagger comments, `writeResponse`, translation.SprintfForRequest
  per project conventions.

## Step 6 — htmx polling partial (render + theme templates)
- Small fragment: `hx-get="/api/jobs/{id}" hx-trigger="every 1s"`, swapped
  in place of the confirm-delete popup once a job starts.
- Stop polling on terminal status (drop the trigger / `HX-Redirect` /
  `HX-Trigger` header on completion).
- No hardcoded colors, ID-selector CSS per component, both themes updated.

## Step 7 — Wire up folder/bulk delete
- `handleAPIDeleteFolder` / bulk delete handler: call `RunAsync` instead
  of blocking `job.RunDeleteFolder`/`RunBulkDeleteFiles` directly.
- Return job id immediately, frontend swaps in polling partial.
- Commit to git earlier/more granularly during the job (not one commit at
  the very end) so an interrupted run leaves a clean committed state
  rather than dangling uncommitted deletions.

## Step 8 — Fix single-file delete (separate, smaller)
- Not async-job based — fix the actual bug: `RefreshCaches()` /
  `InvalidateFileListCache()` wipes the cache synchronously before the
  background rebuild finishes, so the subsequent `/browse` tree fragment
  falls through to a synchronous full-vault walk.
- Fix: either incrementally update the cache (remove just the deleted
  entry) instead of nuking it, or serve stale cache until the background
  rebuild finishes (stale-while-revalidate) in `metadata_cache.go`.

## Step 9 — Follow-up candidates (not now)
- Route other slow ops (import/export, cache rebuild, bulk move/rename)
  through `RunAsync` once the base system is proven on delete.

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
