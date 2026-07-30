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
- loading kanban (/kanban/arbeit-sirconic-todo) takes a really long time => does it load ALL files and not just the collection files?
- kanban server route - all ancestors selection - it always shows all ancestors even if all of its child are already archived - maybe a new env which sets all done/archived status?
- take a look at the whole codebase into all javascript snippets/scripts with the goal of reducing javascript in favor of more htmx - im also fine with refactoring to make this to work since i think we already use a lot of javascript which could be resolved using htmx
- refactor wiki-autocomplete.js to use htmx (at the moment it takes json input) -> maybe we can generalize the navigation in such a output since we need it a few times
- wire the editor settings directly in the editor via a button menu besides the toolbar
- opening admin in new tab - close the sidebar just like for settings and system
- add repair broken links to jobs
- tests
  - manual-trigger test doesnt run through: `Error: RunAsync's manual job sequence did not complete as expected within the timeout`
  - all-editor-types `Expected: 6 editor types, including codemirror-editor and todo-editorActual: 5 types: [filter-editor list-editor todo-editor index-editor codemirror-editor] Error: AllEditorTypes did not return the expected set`
  - in-app-tests.log => also add the error message behind the failed test
  - clean the test folder after the tests
  - is anything besides the test folder influenced? can i run the tests in my prod binary?

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
