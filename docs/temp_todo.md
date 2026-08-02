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
- i like the https://traven.dev/ look with a topbar with settings in the topbar to enable/disable certain features

**per ai**
- not important
  - codemirror copy + paste with y+p does not work properly (e.g. i have something in the clipboard and it doesnt paste and i need to use ctlr+v in edit mode)
    - is a harder problem to tackle
  - backup solution
  - create a system for themes (another repoistory with themes)
    - .e.g. create a table/dict with all top level folders - than check if there is a theme.json
  - deployment
    - make docker build viable
      - for usage
      - for devs
- move file button (can be done in rename at the moment)
- settings/configs to hide certain folders from different searches
- update/change the fontpreview solution (pdfexport) - i dont like it (only setting to touch the DOM structure around the `<select>`)
- handle image links in lists/todo => maybe just show the link? make setting to choos between link/image? at the moment its just invisible
- filter editor - add a save and cancel button
- kanban server route - all ancestors selection - it always shows all ancestors even if all of its child are already archived - maybe a new env which sets all done/archived status?
- wire the editor settings directly in the editor via a button menu besides the toolbar
- add repair broken links to jobs
- autocomplete only shows 20 entries?
- kanban add a refresh button
- run all tests in my prod takes forever => is there a need to get all files multiple times? how can we optimize this? can we use the filter and filter for the test collection so all other files are ignored?
- refactor the build in theme and make the slideout modular => each feature (e.g. tree slideout, search, file history) can be easily plugged in anywhere
- deleting references does not work
- better solution for all of the xxxNoRefresh (cache) functions
- translation for log?
- atomic
  - `handleAPISetMetadata` went from one atomic save to up to six independently-locked field writes, changing the endpoint's failure/atomicity contract (partial success is now possible, and concurrent readers can see an in-between state). Probably fine given the new model, but it's a real behavioral change that isn't called out anywhere.
  - `handleAPIUpdateDashboard`/`handleAPIRenameDashboard` and `handleAPISetMetadata` moved from one atomic-looking (but actually unsynchronized) write to several independently-locked field writes. Each individual field is now race-safe, but a single "set metadata" HTTP request no longer holds one lock across all its fields — a concurrent `MoveCard` could still interleave between, say, the `SetTags` and `SetParents` calls within the same request. That's a real improvement over "no locking at all," just not full request-level atomicity; the comment above `handleAPISetMetadata` acknowledges this honestly, which I'd rather see than a claim of full atomicity that isn't true.

# every other time

- take a look at all routes if we use writeResponse everywhere neccessary and if we can update the functions where we only use json to htmx as well
- take a look at the whole codebase into all javascript snippets/scripts with the goal of reducing javascript in favor of more htmx - im also fine with refactoring to make this to work since i think we already use a lot of javascript which could be resolved using htmx

# ai prompts

## kill 1324

```bash
fuser -k 1324/tcp
```

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
