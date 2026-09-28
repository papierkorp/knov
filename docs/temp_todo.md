# temp todo

# small stuff

- features
  - create a system for themes (another repoistory with themes)
    - e.g. https://github.com/papierkorp/knov_themes
    - e.g. create a table/dict with all top level folders - than check if there is a theme.json
  - multiview in theme
  - a collection/library for books so i can download multiple books with one click
  - toc in codemirror edit all
  - book/index editor - drag and drop
  - system/structure for startup warnings / upgrades to a new version for new releases (breaking changes) (since when, what changed, what does the user have to do now) - maybe use this in the releasenotes instead of breakingchange
    - add the kanban envs
    - add link extraction
  - tournament editor
  - encrypt single files/folders
- fixes
  - if a `.` is in the name its not detected as markdown for new files
- chore
  - change the cleanup orphaned files to look the same as scan for broken links and misplaced media but keep the storage statistics without the cleanup button
  - Pre-existing, not new: docs are read and written with os.ReadFile/os.WriteFile directly, bypassing contentStorage and without the docs lock. That’s the same as updateLinksInFile, so it’s consistent, but a concurrent editor save could race the rewrite.
  - Uppercase attributes. rewriteHTMLAttrRe makes only the tag name case-insensitive. <img SRC="…"> is missed. Using (?i:src|href) would fix it, if you care.
  - tests copies the live data => do we need to copy the whole data folder? so a user needs double the space if he wants to test?
  - Files that already have two status tags keep both. Before, the sanitizer reduced them to one. Now anything in oldTags is kept unchanged, until the user adds a new valid status tag.
  - parseJSON uses (nil, nil) to mean “ignore this value”, and only one caller checks for it. ImportSettingsJSON checks parsed != nil. BulkSetFromForm doesn’t, so it relies on parse never returning nil except for NoteSetting, whose store does nothing. If a future parse returns nil, store will panic on v.(bool). This works today but is fragile. If you touch it again, have store skip nil values.
  - validateKanbanFolder rejects folders whose first segment is docs, media or files. That’s needed because of how pathutils strips or reroutes those prefixes. The side effect is that a real folder called docs/files/... can never be a board. It’s documented in the code comment, but not in the setting’s Desc.
- test
  - remote git in mobile

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

```bash
give me an overview of the current git changes, dont make any changes yet just give me your opinion

- does it use the same principles as the rest of the application/packages?
- is it easy to understand code without overcomplicating it? it should be a easy to follow solution
- is there overengineering going on which could easily be simplified?
- does it fit in the app or is it out of place?
- if you could refactor it - are there better ways to implement it?
- are there some serious problems with the current solution?
- what is it doing exactly?
```

## analyze

```bash
**Role:** Act as a Senior Software Architect and Lead DevOps Engineer with 15+ years of experience in building scalable, production-grade systems. Also act as a pragmatic minimalist who strongly prefers the simplest solution that satisfies the current requirements.

**Task:** Analyze the uncommitted working-tree changes. Critique the implementation, check for architectural consistency, verify production readiness, and explicitly evaluate whether the solution is overengineered and can be simplified.

**Global Rules:**
- Prefer YAGNI, KISS, and boring, proven technology.
- Assume the simplest possible solution is preferred unless a more complex design is justified by a current, concrete requirement.
- Do not recommend abstractions, layers, patterns, dependencies, configuration, or indirection unless they solve a demonstrated problem today. Exception: splitting out a new function or thin wrapper is allowed when it removes flags, branching, or mixed responsibilities from an existing function.
- For every recommendation, ask: Can this be removed, inlined, replaced by a framework/ORM/stdlib built-in, or done with less code?
- Before accepting any new function, helper, or utility, search the codebase for an existing one that already does the same or nearly the same thing. Reusing existing code beats writing new code.
- Do not accept changes that grow an existing function with boolean flags, mode parameters, optional arguments, or special-case branches just to serve one new caller. If that happens, evaluate whether a separate function or thin wrapper would be simpler.
- If complexity is justified, state exactly what current requirement justifies it.
- If the code is already appropriately simple, say so clearly. Do not invent complexity just to fill sections.

**Please provide your analysis in the following five sections:**

1. Implementation Review (The "Right Way")
- Is this the standard, idiomatic way to implement this feature in [Language/Framework]?
- Are there logical errors, edge cases, or security vulnerabilities (e.g., injection, race conditions, improper error handling) in this specific code block?
- Does it violate SOLID principles or common design patterns?
- If it follows SOLID/patterns but adds unnecessary indirection, call that out explicitly.

2. Architectural Consistency (The "Everywhere" Check)
- *Note: Search the repository for related code before answering. If you still lack enough context about the rest of the codebase, ask me for specific files to compare against.*
- Based on the code provided, does this follow a pattern that looks reusable and consistent?
- **Red Flags:** Does this introduce a "one-off" solution? (e.g., using a direct SQL query when the project uses an ORM, or hardcoding values that should be environment variables).
- Suggest how to refactor this to fit a unified architecture if it feels disjointed.
- If the existing architecture is already too complex, say that instead of forcing consistency with it.

**Reuse & Function Boundaries:**
- **Unnecessary new functions:** Do the changes add functions, helpers, or utilities that duplicate existing ones in the codebase, framework, or stdlib? For each, name the existing function (with file path) that should be used instead.
- **Near-duplicates:** Is there an existing function that already does most of the job? Would a small, natural extension of it be better than a copy with minor differences?
- **Overcomplicated existing functions:** Were existing functions modified by adding boolean flags, mode/type parameters, optional arguments, special-case branches, or type checks to support the new use case? If so, would a separate function or thin wrapper be clearer? Consider the impact on existing callers.
- **Decision rule:** Extend the existing function if the change fits its single responsibility and existing callers are unaffected or benefit. Create a new function or wrapper if the change adds a second responsibility, alters behavior for existing callers, or needs a flag to switch between behaviors.
- For each finding, state the verdict: "Reuse existing X," "Extend existing X," "Split into new function," or "Fine as is."

3. Production Readiness Assessment
- **Error Handling:** Is it robust? What happens if the API/Database goes down?
- **Performance:** Are there N+1 query issues, unnecessary re-renders, or O(n) complexity issues?
- **Observability:** Is there logging/metrics in place for this new code?
- **Testing:** Does the structure allow for easy unit/integration testing? (If no tests are shown, point out what tests are missing).
- **Verdict:** Is this "Production Ready," "Needs Minor Refactoring," "Prototype Only," or "Overengineered — Simplify First"?

4. Alternative Architectures
- Propose **one better architectural approach** to solve the same problem.
- If the best alternative is simply a simpler version of the current approach, propose that instead of a more complex architecture.
- Explain the trade-offs (e.g., "This alternative is more scalable but takes longer to implement" or "This is simpler but may not handle multi-region failover").

5. Simplicity & Overengineering Check
- Is this overengineered? Identify any unnecessary abstractions, layers, wrappers, factories, interfaces, configuration, dependencies, generic types, indirection, or premature generalization.
- What is the simplest possible solution that still meets the stated requirements? Show the minimal implementation, diff, or pseudocode.
- What can be deleted, inlined, hardcoded for now, replaced by an existing function in the codebase, replaced by a framework/ORM/stdlib feature, or deferred until actually needed?
- Is the added complexity justified by current scale, reliability, security, or team constraints? If not, simplify.
- Rank the simplifications by impact vs. effort.
- **Simplicity Verdict:** "Already simple," "Can be simplified," or "Significantly overengineered."
```

## review

```bash
Role: Act as a Staff-Level Software Engineer conducting a code review with a high bar for quality and maintainability.

Task: Review the current git diff. You are strictly prohibited from rewriting the code or providing "fixed" code snippets. You are only permitted to give your professional opinion on the changes.

Constraints:
- Do not output any code. Do not suggest code blocks, patches, or refactored versions of the provided diff.
- Verdict: If the changes are completely safe, logically sound, and meet standard best practices, explicitly state: "VERDICT: APPROVED" in your response.
- Problems: If you find any issues, do not fix them. Instead, explain why they are problematic, the potential impact (e.g., runtime error, security hole, performance bottleneck, unreadability), and the strategy to fix them (without writing the actual code).
- Review the changes from two angles:
  1. As an unreleased app that does not need backwards compatibility: breaking changes, simpler designs, removed compatibility shims, and cleanup of legacy paths may be acceptable if they improve the final product.
  2. As an app with backwards-compatibility requirements: existing APIs, data formats, contracts, configuration, persisted state, integrations, and user behavior must continue to work unless a migration or deprecation path is clearly justified.
- If the two angles lead to different conclusions, state that explicitly. If the verdict differs by angle, say so.

Areas to scrutinize (your opinion must cover these):
- Correctness: Are there off-by-one errors, incorrect variable reassignments, or logical flaws?
- Edge Cases: Will this fail on empty arrays, null values, or extreme inputs?
- Security: Does this introduce injection risks, exposed secrets, or unsafe deserialization?
- Performance: Are there O(n²) loops hiding in the changes, or unnecessary database queries?
- Maintainability: Is the naming clear? Is it adding accidental complexity or tight coupling?
- Side Effects: Are there changes to global state, environment variables, or external APIs that werent considered?
- Architecture: Are the changes in line with the rest of the codebase?
- Ignore the i18n translations since they are unrelated.

Also give your opinion about the changes: is the current solution overengineered and can it be simplified?

- run `--start-tests --remove` and check for potentiol bugs
```
