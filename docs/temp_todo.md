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
  - add a tracker editor (e.g. raid clan boss) in edit show a form where i can click, make entries and add new inputs and in view show them as a statistic (makdown table?)
  - let me allow to select what to auto backup/create multiple auto backups
- fixes
  - media rename?
  - releasenotes does not work
- chore
  - add examples / example usage to template_data.md and explain what a template data acutally is and add a link to the create_your_own_theme.md file
  - remove allowing to set the datapath in the admin (should i leave git repository in? if yes we should also add the user/ssh key management and a test connection button otherwise it doesnt make much sense)
  - for all tests: make it so it doesnt affect the live data (e.g. create a new database just for the test or copy the data/media or copy everything into a temp folder (so we have knov binary and the temp folder in the same height))
  - i used: `git remote add origin git@github.com:papierkorp/test2.git && git branch -M main && git push -u origin main` but the app still said i dont have a git remote

# Async follow up jobs

- [ ] job cancellation: thread `context.Context` into `Job.Run()`, add cancel endpoint/button (currently only way to stop a stuck job is restarting the process, which re-runs it via `RecoverInterrupted()`)
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
