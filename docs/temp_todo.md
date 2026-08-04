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
  - backup solution
  - create a system for themes (another repoistory with themes)
    - .e.g. create a table/dict with all top level folders - than check if there is a theme.json
  - deployment
    - make docker build viable
      - for usage
      - for devs
  - update/change the fontpreview solution (pdfexport) - i dont like it (only setting to touch the DOM structure around the `<select>`)
  - wire the editor settings directly in the editor via a button menu besides the toolbar
- refactor the built in theme and make the slideout modular => each feature (e.g. tree slideout, search, file history) can be easily plugged in anywhere
- i like the https://traven.dev/ look with a topbar with settings in the topbar to enable/disable certain features
- todo editor with markdown on top/bottom + header
- no success message / anything after successfull rename and the modal stays up (need to wait quite a bit of time)
- kanban refresh - destroys order
- update hide paths (filevisibility) to only hide from certain features (e.g. hide in tree but show in browse)

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
