# temp todo

# small stuff

- features
  - create a system for themes (another repoistory with themes)
    - e.g. https://github.com/papierkorp/knov_themes
    - e.g. create a table/dict with all top level folders - than check if there is a theme.json
  - implement a 2 view system (e.g. todo list in raw markdown/vs rendered todolist, or the new tracker editor => clicker vs statistics)
  - multiview in theme
  - upgrade path tool in /system/release which shows the changes from one speicific build to another
  - a collection/library for books so i can download multiple books with one click
  - tracker editor
    - give me options to define the output file
    - are entries removed from the json if we remove them from the editor?
    - add 3 dots to the right of the "+" and move the "X" remove button into there
    - add a reset to 0 button (in red) to the 3 dots
    - add tests
  - general solution for backwards compatibiliy scripts (similar to the db migration maybe?)
  - codemirror: add table button
  - info slideout - open file with (another editor)
- fixes
  - **table component HTML bypasses `sanitizeHTML`** - `internal/parser/table.go`'s `RenderTableHTML` (served by the separate `/api/components/table` htmx endpoint) never goes through the new bluemonday-based `sanitizeHTML` in `parser_markdown.go`; it's a different code path from the markdown `Render()` pipeline. Not urgent now, but if this HTML ever gets routed through `sanitizeHTML`, the current allowlist would silently strip its `<select>`/`<option>` filter dropdowns (UGCPolicy disallows those elements) and its `hx-target`/`hx-include` attributes (not in the allowlist) - worth deciding then whether to extend the policy or keep it a deliberately separate trust boundary
- chore
  - dokuwiki example file add all plugins which can be converted as a list with links to the dokuwiki plugins
  - storageinterface for the editors (filter and tracker) or keep them in the config storage?
  - Cross-file heading-id collisions: `renderDocsMarkdown` now builds one TOC per file with independently-scoped dedup (`Headings` makes a fresh `usedIDs` map per call), then concatenates. If two changelog/release files each have a heading like `## Added`, both get id `added` and the combined page ends up with duplicate DOM ids and two TOC entries pointing at the same anchor. I confirmed this isn't a regression — the renderer already deduped ids per-file (`mdHandler.Render` per iteration, each with its own `usedIDs`) before this diff — but it's worth flagging since it's now made more visible/permanent by the accompanying comment ("each file keeps its own heading-id dedup scope"), which documents it as accepted rather than incidental. Not a blocker, just a known limitation now baked in as intentional.

# every other time

- take a look at all routes if we use writeResponse everywhere neccessary and if we can update the functions where we only use json to htmx as well if its useful
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
