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

**per ai**
- not important
  - add s3 to backup storage
  - add go tests?
  - create a system for themes (another repoistory with themes)
    - .e.g. create a table/dict with all top level folders - than check if there is a theme.json
  - deployment
    - make docker build viable
      - for usage
      - for devs
  - update/change the fontpreview solution (pdfexport) - i dont like it (only setting to touch the DOM structure around the `<select>`)
- features
  - combine todo/list editor (setting to change the gfm todo), add headers (which adds a `#` instead of a list)
  - update hide paths (filevisibility) to only hide from certain features (e.g. hide in tree but show in browse)
  - in the theme let me also arrange the info slideout components
- fixes
- chore
  - async jobs follow up candidates


# every other time

- take a look at all routes if we use writeResponse everywhere neccessary and if we can update the functions where we only use json to htmx as well
- take a look at the whole codebase into all javascript snippets/scripts with the goal of reducing javascript in favor of more htmx - im also fine with refactoring to make this to work since i think we already use a lot of javascript which could be resolved using htmx
- pass over css files (components.css/panels.css/layout.css) for dead selectors, confirm remaining ones follow the id-selector convention
- check the whole codebase for hardcoded colors and replace theme with the vars provided by the defaults.css file


# kanban folder sync

optional per-board feature: auto-create one folder per kanban status under a board's base folder and keep files physically moved into the matching status folder. metadata (the `kb-status-*` tag) stays the single source of truth - folder location is a materialized view kept in sync in both directions, never the other way around.

**config**
- extend `KNOV_KANBAN_BOARDS` entries with an optional 3rd colon segment instead of a new env var, e.g. `projects/work:Work Board:sync` - backward compatible, entries without the 3rd segment behave as today
- `getKanbanBoardsEnv` (`internal/configmanager/config.go:265`) currently does `SplitN(pair, ":", 2)` - needs to become `SplitN(pair, ":", 3)`, add `SyncEnabled bool` to `KanbanBoard` (`config.go:72`)

**sync direction 1 - in-app status change (tag -> folder), synchronous**
- hook into the existing "status tag changed" detection in `applyKanbanTimestamps` (`internal/files/metadata.go:171`), which already fires for every write through `MetaDataMutate` (`metadata.go:64`) regardless of call site (drag/drop, editor tag edit, API)
- on change, physically move the file into `<board_folder>/<status_slug>/` for sync-enabled boards - reuse the existing rename + link-update logic (`handleAPIRenameFile`, `internal/server/api_files.go:799`), don't hand-roll a new `os.Rename`
- do the tag write + physical move as one unit under the file's existing per-path lock so no half-moved state is ever observable

**sync direction 2 - out-of-band changes (folder -> tag), async**
- hook into the `file-sync` cron job's git-diff reprocessing (`fileJob`, `internal/job/cronjob.go:20`) - this is the only mechanism that sees changes not made through the app (OS file manager, git push/pull)
- for each changed file under a sync-enabled board, compare its status-subfolder **relative to the board root** against its `kb-status-*` tag; on mismatch (or missing tag), fix the tag via `MetaDataMutate` to match the folder - covers moves and brand-new files created/copied directly into a status folder
- this reconciler must go through `MetaDataMutate` (same lock as direction 1), never write metadata directly - keeps it naturally idempotent/no-op after an in-app move instead of ping-ponging

**edge cases**
- file moved outside the board's folder tree entirely -> remove the `kb-status-*` tag
- board root folder itself renamed/moved -> `KNOV_KANBAN_BOARDS` is static and won't follow it; don't try to auto-detect/rewrite config, just log a startup warning if a configured board folder path doesn't exist on disk
- renaming/moving a status *subfolder along with* the board root is fine as long as the direction-2 rule is "status folder relative to board root", not "did the absolute path change" - it naturally no-ops
- true concurrent edits from two independent sources (e.g. two devices) should route into the existing `Conflict*` metadata fields / conflict handling, not a new bespoke merge scheme
- `archive` counts as a status too (`KNOV_KANBAN_ARCHIVE_STATUS`) - needs its own folder like any other column

## todo

- [ ] add optional 3rd `:sync` segment to `KNOV_KANBAN_BOARDS` parsing, add `SyncEnabled` to `KanbanBoard`
- [ ] auto-create missing status folders (incl. archive folder if `KNOV_KANBAN_ARCHIVE_STATUS` set) under the board root on startup for sync-enabled boards
- [ ] hook physical move into the status-tag-changed detection in `applyKanbanTimestamps`/`MetaDataMutate`, reusing existing rename + link-update logic, only for sync-enabled boards
- [ ] hook async reconciliation into the `file-sync` job: compare status-subfolder vs `kb-status-*` tag per changed file, fix tag via `MetaDataMutate` on mismatch (covers missing tags too)
- [ ] remove `kb-status-*` tag when a file is moved outside the board's folder tree
- [ ] log a startup warning for any `KNOV_KANBAN_BOARDS` folder path that doesn't exist on disk
- [ ] update `docs/configuration.md` kanban section and `.env.example` with the new `:sync` flag
- [ ] manual QA: drag card in UI moves file on disk; move file via OS file manager updates tag after next file-sync run; new file created directly in a status subfolder gets tagged; file moved out of board tree loses its tag; renaming the board root folder logs a warning and doesn't corrupt tags

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
