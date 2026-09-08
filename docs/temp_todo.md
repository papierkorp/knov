# temp todo

# small stuff

- features
  - create a system for themes (another repoistory with themes)
    - e.g. https://github.com/papierkorp/knov_themes
    - e.g. create a table/dict with all top level folders - than check if there is a theme.json
  - add a tracker editor (e.g. raid clan boss) in edit show a form where i can click, make entries and add new inputs and in view show them as a statistic (makdown table?)
  - cache for books?
  - todo editor add a date at the end for each click
  - implement a 2 view system (e.g. todo list in raw markdown/vs rendered todolist, or the new tracker editor => clicker vs statistics)
  - multiview in theme
  - upgrade path tool in /system/release which shows the changes from one speicific build to another
- fixes
  - 
- chore
  - subheader: `## 1. Versuch - per HAProxy und public IP` is not editable
  - where does test/editor-tests/edtest-filter.md and example_filter.md come from? i feel like they are randomly there without me running the tests
  - s3 backup tests?

# Async follow up jobs

- [x] job cancellation: thread `context.Context` into `Job.Run()`, add cancel endpoint/button (currently only way to stop a stuck job is restarting the process, which re-runs it via `RecoverInterrupted()`) - scoped to bulk-delete-files/delete-folder (the only StartAsync jobs with a plain per-item loop safe to interrupt); full-rebuild would need cancellation checks threaded through several `files` package internals, restore is destructive/self-restarting so not offered
- [x] full-rebuild cancellation: wire the `ctx` `fullRebuildJob.Run` already receives (currently discarded) into `files.MetaDataLinksRebuild` - it's the actual bottleneck of a full rebuild (5 sequential passes over every doc/media file, disk read + parse + DB write per file), dwarfing the other 4 steps `fullRebuildJob.Run` calls. Needs a `ctx context.Context` param added to it plus a `ctx.Err()`/break check in each of its 5 loops (same pattern as `BulkDeleteFiles`), and its other call sites updated (`job/cronjob.go`'s scheduled rebuild, `test/editorstest/testcases_fileops.go`, `test/testdata.go`). `MetaDataInitializeAll` is a distant second candidate (only costly on a mostly-uninitialized vault). Not worth it for `MetaDataPurgeStale`/`MetaDataPurgeDuplicates`/`UpdateOrphanedMediaCache` - all bulk-read-bound and cheap, and `UpdateOrphanedMediaCache` has the widest blast radius of call sites (5 others) for the least benefit; then add `full-rebuild` to `job.IsCancellable`
- [x] sqlite `jobs` table cleanup: rows in `jobStorage` are never purged, grows unbounded; add a retention/purge job similar to existing `notification-purge` cron job - added `jobStorage.Purge(maxCount, maxAgeDays)` (sqlite, never touches `running` rows), a `job-history-purge` job (200 rows / 30 days), and wired it into `RunAsync()`'s "run all jobs now" sequence before `notification-purge`. No dedicated ticker (notification-purge has none either)
- [x] unify job history: cron/manual jobs use a 50-slot in-memory ring buffer (`job/history.go`, lost on restart), async jobs use sqlite (`jobStorage`); `/system/jobs` only shows the in-memory one, so async job history is invisible there - added `jobStorage.List(limit)` (newest-first, any status), a `JobRun.ID` field (set only for StartAsync runs, threaded through `runLocked`/`recordStart`) so a ring-buffer entry can be matched to its jobStorage row, and `job.GetHistory(limit)` which merges the two (skipping storage rows already in the buffer by ID) and feeds `/api/system/jobs`. Also added `JobStatusInterrupted` + a `.job-status-interrupted` CSS rule so `RecoverInterrupted`'s marked-interrupted rows render correctly instead of falling through as "error". Verified via the `jobs`/`async-job` in-app test suites and a manual `/api/system/jobs` check showing pre-restart async rows (including an `interrupted` one) that the old ring-buffer-only view would've hidden
- [x] progress reporting: added a `job.Progress` atomic counter + `Progresser` mixin (alongside `Outputter`/`Messenger`), an `asyncJobs` id->Job registry next to `asyncCancel`, and `job.GetProgress(id)`. The three slow loops (`files.BulkDeleteFiles`, `files.BulkUpdateMetadata`, `files.MetaDataLinksRebuild`) take an optional `func(done, total int)` reporter (variadic - existing/test call sites untouched); bulk-delete/delete-folder, bulk-update-metadata and full-rebuild jobs pass `j.prog.Report` through. `RenderJobStatus` now shows `working... 340/1200` while a reporting job runs, plain `working...` otherwise. full-rebuild total is `4*len(paths)` (the four doc passes; the two media tail passes aren't counted)
  - review follow-ups (non-blocking, from staff-level review of the diff):
  - [ ] full-rebuild bar pins at `N/N` during the two media tail passes + the other steps `fullRebuildJob.Run` runs after `MetaDataLinksRebuild`, so a user can read it as hung. Options: count the two media passes into the total too, or have `fullRebuildJob.Run` drive a coarser step counter (`step 3/7 ...`) instead of proxying only the links-rebuild sub-progress
  - [ ] `RenderJobStatus` reaches into `job.GetProgress(id)` itself, unlike its already-resolved `cancellable bool` param. For symmetry, resolve `(done, total, ok)` in the handler for GET /api/jobs/{id} and pass them in, keeping render a pure string builder
  - [ ] `Progress.Load()`'s "can never report done > total" godoc only holds because every current caller passes a constant `total`. Either enforce it (clamp/monotonic total) or soften the comment so a future varying-total caller isn't misled
  - [ ] cosmetic: `report(i+1, len)` / `bump()` fire before the item is processed, so the bar hits `N/N` while the last file is still in flight (and stays there if that last op fails). Move the report to after the work if it bothers you
  - [ ] add the `working... %d/%d` translation key to the translation files (currently only the plain `working...` key exists)
- [ ] cross-session completion notification (cheap version only): on an async job's terminal status, also `notificationStorage.Add(...)` so other open tabs surface it on next page load - reusing the exact pattern `markInterrupted` already uses. No live push (SSE) - not worth it for a single user. (dropped from the list: manual retry for failed async jobs, and global concurrency/queueing - both multi-user/high-load infra that doesn't apply here)

# every other time

- take a look at all routes if we use writeResponse everywhere neccessary and if we can update the functions where we only use json to htmx as well
- take a look at the whole codebase into all javascript snippets/scripts with the goal of reducing javascript in favor of more htmx - im also fine with refactoring to make this to work since i think we already use a lot of javascript which could be resolved using htmx
- pass over css files (components.css/panels.css/layout.css) for dead selectors, confirm remaining ones follow the id-selector convention
- check the whole codebase for hardcoded colors and replace theme with the vars provided by the defaults.css file
- add all missing german translations

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
- what is it doing exactly?

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
- Ignore the i18n translations since they are unrelated
- Ignore the temp_todo.md file this is just a summary for me

Also give your opinion about the changes, is the current solution overengineered and can be simplified?
