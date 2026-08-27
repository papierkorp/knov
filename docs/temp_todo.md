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
  - new "book" feature with different stages
    - select headers/anchors from different files and export into one markdown/pdf
    - select different files and export into one markdown/pdf
- fixes
  - media rename?
- chore
  - async jobs follow up candidates
  - switch to static id instead of using the path
  - in the logs sidebar use a "summary" or "short" version or something like this with: time + level + message only + a parameter for a refresh button on top and use the all (merged)
  - in the backup sidebar use a "summary" or "short" version or something like this with: time + event only + a parameter to optionally add the action buttons (but dont show the buttons for now)
  - add examples / example usage to template_data.md and explain what a template data acutally is and add a link to the create_your_own_theme.md file

# every other time

- take a look at all routes if we use writeResponse everywhere neccessary and if we can update the functions where we only use json to htmx as well
- take a look at the whole codebase into all javascript snippets/scripts with the goal of reducing javascript in favor of more htmx - im also fine with refactoring to make this to work since i think we already use a lot of javascript which could be resolved using htmx
- pass over css files (components.css/panels.css/layout.css) for dead selectors, confirm remaining ones follow the id-selector convention
- check the whole codebase for hardcoded colors and replace theme with the vars provided by the defaults.css file

# backup/restore testcases

internal/test/backuptest doesn't cover the storage migration/cross-backend restore changes yet - next agent should extend it:

- add a case for cross-backend restore (backup.Migratable/RestoreMigrate): seed a probe, back up, switch provider (metadataStorage: json/sqlite/yaml, kanbanStorage: json/sqlite), restore, verify the probe converted correctly into the new backend
- add a case asserting a non-Migratable storage (chat/notification/search/config) fails restore with an explicit "cannot auto-convert between backends" error when the manifest's recorded backend differs from the current one, instead of silently applying mismatched data
- add a case for an old-format backup with no manifest "Backends" entry (pre-migration backups) still restoring fine via the existing same-backend path
- add a case for backup.Restore's afterRestore/touched contract: a restore that fails before touching anything (bad set name, corrupt archive) must not fire afterRestore; a restore where every storage's RestoreMigrate reports untouched must not fire it either
- add a case for kanbanStorage's "noop" short-circuit: a backup taken while kanban was disabled restored onto an enabled backend (and vice versa) is a no-op, not an error
- add a case for yamlFrontmatterStorage.Backup now snapshotting real front matter into a scratch sqlite file (used to be a no-op) - confirm restoring a "yaml"-tagged backup onto sqlite/json converts it via RestoreMigrate
- add a case confirming a restore refreshes the cache (files.CacheInvalidate via job.restoreJob's afterRestore, files.RebuildAllCaches via restoreAndReinit) - cache lost its own probe/backup/restore in this change since it's no longer a registered storage, so nothing currently checks it gets refreshed

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
- Ignore the i18n translations since they are unrelated
- Ignore the temp_todo.md file this is just a summary for me

Also give your opinion about the changes, is the current solution overengineered and can be simplified?
