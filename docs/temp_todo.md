# temp todo

# small stuff

- features
  - create a system for themes (another repoistory with themes)
    - e.g. https://github.com/papierkorp/knov_themes
    - e.g. create a table/dict with all top level folders - than check if there is a theme.json
  - add s3 to backup storage
  - add a tracker editor (e.g. raid clan boss) in edit show a form where i can click, make entries and add new inputs and in view show them as a statistic (makdown table?)
  - add a image to the testfiles so setup/clean testdata also includes it
  - cache for books?
  - table filter add filter as soon as you find the same 2 times
- fixes
  - releasenotes does not work
- chore
  - dashboard one column widget if only one widget - take the whole space to the bottom
  - change index/book editor to use markdownlinks instead of wiki links
  - create/generalize a robust reuseable markdown scanner, at the moment we have duplicates for markdown/books and a parser in the book package directly to catch both ``` and ~~~
  - we have two autocomplete-wiring mechanisms which coexist: `GenerateDatalistInput` was modernized to `data-path-autocomplete` + global auto-init, but `GenerateDatalistInputWithSave` still emits the old inline script. Worth a follow-up to converge.
  - make books more handwritten save

# Async follow up jobs

- [x] job cancellation: thread `context.Context` into `Job.Run()`, add cancel endpoint/button (currently only way to stop a stuck job is restarting the process, which re-runs it via `RecoverInterrupted()`) - scoped to bulk-delete-files/delete-folder (the only StartAsync jobs with a plain per-item loop safe to interrupt); full-rebuild would need cancellation checks threaded through several `files` package internals, restore is destructive/self-restarting so not offered
- [ ] full-rebuild cancellation: wire the `ctx` `fullRebuildJob.Run` already receives (currently discarded) into `files.MetaDataLinksRebuild` - it's the actual bottleneck of a full rebuild (5 sequential passes over every doc/media file, disk read + parse + DB write per file), dwarfing the other 4 steps `fullRebuildJob.Run` calls. Needs a `ctx context.Context` param added to it plus a `ctx.Err()`/break check in each of its 5 loops (same pattern as `BulkDeleteFiles`), and its other call sites updated (`job/cronjob.go`'s scheduled rebuild, `test/editorstest/testcases_fileops.go`, `test/testdata.go`). `MetaDataInitializeAll` is a distant second candidate (only costly on a mostly-uninitialized vault). Not worth it for `MetaDataPurgeStale`/`MetaDataPurgeDuplicates`/`UpdateOrphanedMediaCache` - all bulk-read-bound and cheap, and `UpdateOrphanedMediaCache` has the widest blast radius of call sites (5 others) for the least benefit; then add `full-rebuild` to `job.IsCancellable`
- [ ] sqlite `jobs` table cleanup: rows in `jobStorage` are never purged, grows unbounded; add a retention/purge job similar to existing `notification-purge` cron job
- [ ] unify job history: cron/manual jobs use a 50-slot in-memory ring buffer (`job/history.go`, lost on restart), async jobs use sqlite (`jobStorage`); `/system/jobs` only shows the in-memory one, so async job history is invisible there
- [ ] progress reporting: add a `Progress() (current, total int)` mixin (alongside existing `Outputter`/`Messenger`) so bulk-delete/bulk-update jobs (which already track counts internally) can surface progress instead of a binary running/done state
- [ ] retry for failed async jobs: `StartAsync` jobs that end in `error` have no retry action; add a manual retry that re-invokes with the persisted args, reusing the existing `Resumable` machinery
- [ ] concurrency/queueing: currently only per-job-type dedup via mutex, no global max-parallel-jobs limit or priority
- [ ] cross-session completion notification: job results only reach the client that's still polling; other sessions/devices don't get notified on completion

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
