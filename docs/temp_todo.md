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
  - add s3 to backup storage
- fixes
  - can we make restart an actual restart or do we rename it to shutdown?
  - codemirror editor => using tab to indent a list entry jumps to save section
  - codemirror editor => using space/delete removes list entries
- chore
  - move copy-code.js to the app
  - storage/config/theme/xxx.json => do we need to add escapes in this json file? i dont like it
  - move translation and changelog to a tools/ package just like templatedocs
  - async jobs follow up candidates


# every other time

- take a look at all routes if we use writeResponse everywhere neccessary and if we can update the functions where we only use json to htmx as well
- take a look at the whole codebase into all javascript snippets/scripts with the goal of reducing javascript in favor of more htmx - im also fine with refactoring to make this to work since i think we already use a lot of javascript which could be resolved using htmx
- pass over css files (components.css/panels.css/layout.css) for dead selectors, confirm remaining ones follow the id-selector convention
- check the whole codebase for hardcoded colors and replace theme with the vars provided by the defaults.css file

# backup-solution — DONE

in the backup.go file we still have a gfs comment => we dont use gfs anymore

can we rename the files to make a bit of sense?
e.g. each backupstorage gets its own file, a storage_interface.go file, should we name the second interface besides sqlite - json or keep file or create both and use the logic of file in the json interface?, or rename target.go to logging.go or events.go or something like this and run.go is also a mix of logs/events and the interface?

also use the date config for the backups
move the logit out of the job package and use it just as a wrapper

does windows support os.createtemp? (atomicwrite.go)



Goal: reliable, easy-recovery backups for StoragePath (metadata/cache/chat/kanban/notifications/config/search) — DataPath is already covered by git. List backups on a new page, restore any of them, keep the set trimmed automatically. contentStorage skipped for now (git already covers it).

Decided: interface-per-storage, not pure filesystem-level. New `internal/backup` package holds the type-aware helpers (`BackupSQLite`/`RestoreSQLite` via `VACUUM INTO`, `BackupFile`/`RestoreFile` for json/yaml) — same shape as `dbmigration`. Each storage interface gets thin `Backup(destDir) error` / `Restore(srcDir) error` methods (one-line bodies delegating to the shared helper), including a no-op on `kanbanStorage_noop.go`.

## prerequisites — DONE
- `configStorage_json.go`, `metadataStorage_json.go`, `cacheStorage_json.go` write via plain `os.WriteFile` (truncate+write), not atomic — switch to temp-file-then-rename before backup can safely copy them live (a backup mid-write could otherwise read a torn file)
- done via new `utils.WriteFileAtomic` (temp file in the same dir + rename), used by all three. `metadataStorage_yaml.go` left untouched on purpose — its writes land in DataPath (docs files), out of backup's scope.

## design decisions
- sqlite `Backup()` takes the storage's existing `sync.RWMutex` `RLock` around `VACUUM INTO` for a consistent per-storage snapshot — no new locking primitive needed
- no cross-storage atomicity — storages are snapshotted sequentially, each under its own lock, not one transaction. fine for single-user use, said so on the `/system/backup` page rather than implying a single point-in-time snapshot
- restore does not hot-swap live `*sql.DB`/in-memory state — write restored files to disk, then reuse the existing restart pattern (`os.Exit(0)` after a short delay), same as the "data path changed, needs restart" flow
- partial backup failure (storage N of M fails mid-run) aborts and discards the whole backup rather than listing an incomplete one — a broken backup that looks valid is worse than no backup
- registration is via self-registering `init()` per storage package into `backup.Register(name, ...)` (mirrors `externalsuite.go`'s `suiteRunners`) — `internal/backup` must not import storage packages directly, since every storage imports `internal/backup` for the helpers, so an orchestrator-side import would cycle
- `metadataStorage_yaml.go`'s `Backup`/`Restore` are no-ops — its data lives in DataPath, already covered by git

## implementation — DONE
- `internal/backup`: `backup.go` (Storage interface, registry, `BackupSQLite`/`RestoreSQLite`/`BackupFile`/`RestoreFile`), `target.go` (`BackupTarget` interface + `localTarget`, byte-stream shaped via `io.Reader`/`io.ReadCloser` so S3/NFS can be added later without touching `Run`/`Restore`), `run.go` (`Run`/`Restore` orchestration, tar.gz staging + extraction, zip-slip guard), `rotate.go` (GFS: ≤7 days kept in full, 1/day to 30 days, 1/month to 365 days)
- every storage package (config, metadata × json/yaml/sqlite, cache × json/sqlite, chat, kanban × sqlite/noop, notifications, search) got `Backup`/`Restore` on its interface + backend(s), plus a package-level `backupAdapter` registered in `init()`
- `internal/job/backup.go`: `RunBackup`/`RunRestore`/`ListBackups`, own dedup mutexes in `scheduler.go`, `DefaultBackupTarget()` = `StoragePath/backups` (local fs)
- API: `GET/POST /api/system/backups`, `POST /api/system/backups/{name}/restore` in `api_backup.go`, swagger regenerated
- `/system/backup` page via `render.HandleSystemBackup` (same inline-HTML-string pattern as `/system/jobs`/`/system/logs`, no per-theme template needed), linked from `admin.gohtml` and the builtin theme's info rail

## selective backup + scheduled auto-backup — DONE
- `backup.Run(target, names...)` takes an optional storage-name selection (empty = everything); each set gets a `manifest.json` listing what it contains, read back via `backup.Manifest` without extracting the rest. `Restore`'s existing per-storage "skip if subdirectory missing" logic already handles restoring a partial set correctly, no changes needed there
- `/system/backup` page: a checkbox row (`job.RegisteredStorageNames()`), all checked by default, submitted as repeated `storages` form values to `POST /api/system/backups`; the list table gained a "Contents" column (`full` vs the actual partial selection)
- automatic backups: `KNOV_BACKUP_AUTO_ENABLED` (bool, default off) + `KNOV_BACKUP_AUTO_INTERVAL` (duration string, default `24h`) — AppConfig env vars, not live Settings (reconsidered after first pass used Settings and the user wanted env vars instead, consistent with the rest of the interval-style cronjob config like `KNOV_CRONJOB_INTERVAL`)
- `job.checkAutoBackup` (ticked every fixed 15m via `job.Start()`, itself unaffected by the interval setting) is a no-op unless enabled and due — "due" is computed from the newest existing set's own timestamp (`backup.ParseSetTime`) rather than separately persisted state, so nothing new needed persisting
- pre-restore safety snapshot (`backup.Restore`'s first step) is always a **full** backup regardless of what's being restored - a partial restore shouldn't leave the safety net partial too
- a partial set's name gets a `_<storages>` suffix (e.g. `2026-08-08T21-10-29_metadata`) so it's identifiable from the filename alone, not just the manifest/Contents column - `ParseSetTime` (used by `Rotate`/`RotateCount`/`checkAutoBackup`) now only parses the fixed-width leading timestamp instead of requiring an exact full-string match, so suffixed names still rotate/get compared correctly
- **bug caught while adding this**: `Rotate` was still calling `time.Parse(nameLayout, n)` directly instead of the new `ParseSetTime`, so every partial-name set silently failed to parse and was never rotated - fixed by routing both `Rotate` and the new `RotateCount` through a shared `listParsedSets`/`deleteSets` pair that both use `ParseSetTime`. Verified: created a partial set + a full one with `KNOV_BACKUP_ROTATION_COUNT=1`, confirmed the partial one gets trimmed correctly
- second rotation strategy: `KNOV_BACKUP_ROTATION_STRATEGY=gfs` (default) or `count` (`KNOV_BACKUP_ROTATION_COUNT`, default 10) - same idea as `logging_rotate.go`'s N-most-recent shifting, for anyone who finds the GFS date-bucketing hard to reason about. `job.rotateBackups` dispatches, unknown values warn and fall back to gfs

## rotation redesign: drop GFS, 3 independent rules + locking — DONE
GFS's date-bucketing thinned old sets down to one per day/month, but that bucket was shared between full and partial sets — a same-day partial ("just metadata") backup could win the slot and silently evict the day's only full backup on the next rotation. Not worth the complexity for how little data a single-user app's backups actually hold, either. Replaced both `Rotate` (GFS) and `RotateCount` with a single `backup.Rotate(target, keepDays, keepFull)`:
- a set is kept if it matches *any* of three independent rules: locked, within `keepDays` of now (any kind), or a full backup among the `keepFull` most recent full backups (a long-term floor, counting every kept full set toward it regardless of which rule kept it)
- partial sets get no long-term floor of their own — once they age out of `keepDays` and aren't locked, they're deleted, so they can never occupy a full backup's slot again
- `backup.IsFullSet(name)` replaces the old manifest-length comparison for "is this a full backup" — checks for the `_<storages>` suffix's underscore in the name itself (the timestamp layout never contains one), cheaper than opening the archive and correct even if a storage is later renamed/removed
- `KNOV_BACKUP_ROTATION_STRATEGY`/`KNOV_BACKUP_ROTATION_COUNT` replaced by `KNOV_BACKUP_ROTATION_KEEP_DAYS` (default 7) / `KNOV_BACKUP_ROTATION_KEEP_FULL` (default 10)
- new lock/unlock: `BackupTarget` gained `Lock`/`Unlock`/`Locked`, implemented on `localTarget` as a `<name>.locked` empty marker file next to the archive (checked by `Rotate` before either count/day rule, not stored in the archive itself since `Rotate` needs it without decompressing anything). `job.LockBackup`/`UnlockBackup` + `POST`/`DELETE /api/system/backups/{name}/lock` + a per-row Lock/Unlock button on `/system/backup`
- `job.ListBackups` (names only) kept for `checkAutoBackup`'s cheap "latest name" need; new `job.ListBackupInfo` (`BackupInfo{Name, Full, Locked}`) feeds the API/render layer instead, so the list page doesn't need a manifest fetch per row just to show "full"

## log-style page + download link — DONE
`BackupInfo`/`ListBackupInfo` above only reflected whatever currently exists on the target - a set rotated away (or referenced by a restore) just vanished, and a restore's own record only ever lived in the in-memory `job.GetRecentRuns` history, which a restore's own restart immediately wipes - the exact moment you'd want "was that restore actually applied?" answered durably.
- `BackupTarget` gained `LogEvent(kind, set)`/`Events()` - a durable, append-only record (`backup.Event{Kind, Set, Time}`) kept independent of whether the referenced set still exists. On `localTarget`, a `log.json` array next to the archives, rewritten atomically on each append; no extra locking needed since `Run`/`Restore` (the only callers) are already serialized under `job.backupMu`
- `backup.Run` logs an `EventBackup` after a successful `target.Write`; `backup.Restore` logs an `EventRestore` after every storage's been restored - both best-effort (a logging failure warns, doesn't fail the backup/restore itself)
- `BackupInfo`/`ListBackupInfo` replaced by `BackupLogEntry{Kind, Set, Time, Available, Full, Locked}`/`job.ListBackupLog` - merges the event log with the target's current `List()`/`Locked()` state, plus a synthetic "backup created" row (timestamped from the name) for any set that predates event logging or survived a lost log file, so nothing already on disk silently disappears from the log
- `/system/backup` (`render.RenderBackupLog`) renders one row per event, newest first: "Backup created: `<name>`" or "Restored: `<name>`", a Contents column, and Lock/Download/Restore buttons - or, once `Available` is false, just an italic "no longer available" note with no actions. Lock/unlock and restore work identically for either kind of row, since both ultimately point at the same underlying archive
- new `job.OpenBackup` + `GET /api/system/backups/{name}/download`, streamed via `setAttachmentFilename` (same helper the logs download uses) rather than `writeResponse`, since this is a raw file stream, not a json/html-negotiated response
- verified end-to-end against a scratch `KNOV_STORAGE_PATH`/port: created a full + a partial backup, locked the full one, downloaded and `tar -tzf`'d the archive to confirm it's valid, restored the full one (confirmed the process exits, `go run` doesn't restart it - expected, no supervisor here), restarted manually and confirmed all 4 rows (2 backups + the pre-restore safety snapshot + the restore itself) survived with correct timestamps and the lock intact, then deleted a set's `.tar.gz` directly and confirmed its row switches to "no longer available" with actions gone

## not done / follow-up candidates
- no in-app test suite yet — see `## test suite — todo` below
- no manual "delete backup" action on the page — rotation trims automatically, kept out to stay minimal (locking now covers "never auto-delete this one")
- BackupTarget only has the local-filesystem implementation; S3/NFS deliberately deferred
- `job.checkAutoBackup`'s "due" check wasn't observed firing live end-to-end — `job.Start()` (and therefore the new ticker) is itself delayed 5 minutes after startup by `main.go`, pre-existing behavior not touched here. Verified the logic by exercising every sub-step individually instead (env values read correctly, `ListBackups`/`ParseSetTime`/`RunBackup` all already covered by other smoke tests) — worth an actual live/test-suite check per the todo below

## bug found during smoke-testing (fixed)
- every sqlite storage's `Restore` failed on Windows with a sharing-violation ("Zugriff verweigert") when overwriting its live db file, because the running process still had it open (WAL mode) — `os.Rename`/overwrite over an open handle is rejected on Windows, unlike POSIX
- fixed by closing each storage's `*sql.DB` right before its `Restore` overwrites the file — safe only because a successful restore always restarts the process immediately after, so the handle is never used again
- verified end-to-end (build binary, run it in a scratch `KNOV_STORAGE_PATH`, create a backup, restore it, confirm the process exits and the restored files load cleanly on relaunch)

## bug found during review (fixed)
- the Windows-sharing-violation fix above closed the live db handle *before* reading/validating the backup file being restored — a corrupt or missing backup entry then failed after the handle was already closed, bricking that storage (every subsequent request to it fails) without the app restarting to recover, since only the success path triggers `os.Exit`
- fixed by splitting `RestoreSQLite` into `ReadSQLiteBackup` (read + magic-byte validation, no db touched) and `RestoreSQLite(data, destPath)` (write only); every sqlite storage's `Restore` now calls `ReadSQLiteBackup` first and only closes its db handle once that succeeds

## KNOV_BACKUPS_PATH — DONE
`job.DefaultBackupTarget()` was hardcoded to `StoragePath/backups` — no way to point backups at separate storage (e.g. a different disk/mount than what they're backing up, the whole point of a backup). Added `KNOV_BACKUPS_PATH` (AppConfig, default `./backups`, sibling of `storage/`/`data/`/`logs/` like every other `KNOV_*_PATH`) and switched `DefaultBackupTarget` to read it directly instead of joining onto `GetStoragePath()`.

## use the async job — DONE

`job.RunRestore` converted from a blocking `execute()` call to `StartAsync` (same mechanism
`delete-folder`/`bulk-delete-files`/`metadata-full-rebuild` already use), added to `asyncdelete.go`'s
shared `resumers` map as `JobTypeRestore` (args = `{"setName": ...}`, rebuilt via the existing
`resolveExistingSet`). `restoreJob.Resumable() bool { return true }` - re-applying the same backup
set to a storage is idempotent, so a crash mid-restore is safely replayed from scratch by
`RecoverInterrupted` on next boot instead of just getting marked interrupted.
- `handleAPIRestoreBackup` now returns a `jobStorage.JobRecord` + `render.RenderJobStatus` polling
  fragment immediately instead of blocking - the backup page's Restore button already targeted
  `#backup-status`, so no template change needed, it now shows the same spinner/poll pattern
  delete-folder's row does.
- `handleAPIGetJobStatus` (`api_jobs.go`) got a `JobTypeRestore` done-case: a toast, no redirect -
  restore's `Run()` still exits the process ~500ms after marking itself done (unchanged from
  before), so there's nothing meaningful to redirect *to*; a still-connected client just sees the
  toast before the connection drops.
- Verified end-to-end against a scratch `KNOV_STORAGE_PATH`/`KNOV_BACKUPS_PATH`/port (run from that
  directory, not the repo root, so it doesn't pick up the repo's own `.env`): created a backup,
  POSTed restore, confirmed the response came back in ~250µs with a job id (not blocking), and the
  log showed the safety snapshot + restore both completing before the process exited to restart.

## test suite — DONE
New `internal/test/backuptest` package, same shape as `settingstest` (`test.Register` + `job.RegisterSuiteRunner("backup-test", ...)`), wired into `main.go`/`server.go`/`api_tests.go`/`admin.gohtml` and `job.RunBackupTest` like every other suite:
- `backup.Run` produces a set containing every registered storage's subdirectory with real content — seed one entry per storage first (a metadata key, a kanban event, a chat message, etc.), then assert it's present after extracting the resulting archive
- `backup.Run` aborts and discards the whole set when one registered storage's `Backup` errors mid-run — no partial `.tar.gz` left on the target afterward
- `backup.Restore` (the package function, not `job.RunRestore`) roundtrips: backup → mutate/delete the seeded data → restore → assert the original data is back. Safe to call directly in-process since only the `job` wrapper (`restoreJob.Run`) triggers `os.Exit`
- `backup.Rotate(target, keepDays, keepFull)` against a target seeded with synthetic past-dated set names (not real timestamps): a full set older than both `keepDays` and outside the `keepFull` floor is deleted; a partial set outside `keepDays` is deleted even if a full set that old would have been kept by the floor rule; a locked set (via a fake `Locked` target or the real marker file) survives regardless of age; include at least one partial (`_metadata`-suffixed) name in the mix, guarding against the `ParseSetTime` parsing bug already caught once
- `backup.Run(target, "metadata")` (selective): resulting set's manifest is exactly `["metadata"]`, only `metadata/` exists in the extracted archive, and `backup.Restore` on it leaves every other storage's live data untouched
- `backup.ReadSQLiteBackup` rejects a corrupt/truncated db file *before* any db handle is touched — regression test for the close-before-validate bug caught during review
- `job.checkAutoBackup`: disabled → no-op; enabled + no existing sets → creates one; enabled + a fresh existing set → no-op; enabled + a synthetic old-dated set on the target → creates one. Call the unexported function directly (same package, `internal/test` suites for other packages already do the equivalent via exported entry points) rather than waiting on the real ticker/`job.Start()`'s 5-minute startup delay
- `job.ListBackupLog`: an event referencing a deleted set comes back with `Available: false` and zeroed `Full`/`Locked`; a set on the target with no matching event still gets a synthetic row timestamped from its name; entries come back sorted newest-first even with sub-second ties (backup vs. its own restore's log entry, always microseconds apart)
- `backup.Event` round-trips through `LogEvent`/`Events` on a fresh `localTarget`, including across a simulated restart (new `localTarget` instance pointed at the same root)
- `utils.WriteFileAtomic`: a reader racing a concurrent write never observes a torn/partial file
- `job.RunRestore`/the actual restart is out of scope for an in-app suite, same precedent as `settingstest`'s data-path-change exclusion (needs a real process restart to verify, can't safely trigger+reverify in-process)

**deviations from the plan above, decided during implementation:**
- every case that creates real archives targets a throwaway `backup.NewLocalTarget(os.MkdirTemp(...))` instead of the real `KNOV_BACKUPS_PATH`, so nothing this suite does ever shows up on `/system/backup` or gets mixed into real rotation. `checkAutoBackup`/`ListBackupLog` (which always target the default target internally) get this via two new in-memory-only setters, `configmanager.SetBackupsPath`/`SetBackupAutoEnabled` — no `.env` write, restored via `defer` at the end of each case
- **found while implementing, not in the original plan**: calling `backup.Restore` directly in-process (as this section's "safe to call directly" note assumed) actually closes every sqlite storage's live `*sql.DB` — that's the Windows-sharing-violation fix's own doc comment ("this connection is never used again in this process"), which is only true because `job.RunRestore`'s caller always restarts right after. Calling it from a long-running suite without restarting would silently brick metadata/cache/chat/kanban/notifications/search for the rest of that dev server's uptime. Fixed by having every case that calls `backup.Restore` go through a new `restoreAndReinit` helper that re-runs each affected storage package's `Init(...)` afterward (mirrors `main.go`'s own startup sequence) instead of calling `backup.Restore` bare
- the abort-on-failure case needed a way to force one storage's `Backup` to fail without touching any real storage — added `backup.Unregister(name)` so a fake `failingStorage` can be registered under a scratch-only name and removed again via `defer`, instead of permanently corrupting every future full backup
- `job.checkAutoBackup` stayed unexported; added a one-line exported wrapper `job.CheckAutoBackupNow()` in `internal/job/backup.go` for the suite to call, consistent with how every other suite reaches otherwise-private job logic through an exported entry point
- verified: `go build ./...`, `go vet ./...`, `swag init` regenerated cleanly with no errors

# ai prompts

## docs

small, precise and concise, high level overview, no examples that are prone to change, just a few bullet points, as few subheaders as possible (i think it becomes more unreadable if its too segmented)

## overview

give me an overview of the current git changes, dont make any changes yet just give me your opinion

- does it use the same principles as the rest of the application/packages?
- is it easy to understand code without overcomplicating it? it should be a easy to follow solution
- is there overengineering going on which could easily be simplified?
- does it fit in the app or is it out of place?
- if you could refactor it - are there better ways to implement it?
- are there some serious problems with the current solution?

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
