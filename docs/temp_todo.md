# temp todo

# small stuff

- features
  - create a system for themes (another repoistory with themes)
    - e.g. https://github.com/papierkorp/knov_themes
    - e.g. create a table/dict with all top level folders - than check if there is a theme.json
  - multiview in theme
  - a collection/library for books so i can download multiple books with one click
  - system/structure for startup warnings
  - tournament editor
  - encrypt single files/folders
  - reset/copy todo list
- fixes
  - kanban - adding new files via http://localhost:1325/files/new/codemirror?prefillpath=arbeit/sirconic/todo/ (the + button in the kanban board) + autotag (e.g. to inbox) doesnt put the files in the todo/inbox => see the screenshot for the settings
  - scan for orphaned media shows all media files
- chore
  - filter: select field parent
  - connections - parents - link shows to /browse instead of /files
  - bulk update - add parent
  - bulk update - remove parent
  - index/book editor => use markdown links instead of wiki links
  - admin action scan for broken parents/grandparents (e.g. i have grandparent for a file that no longer exists? how is this even possible?) and why is the file not shown as a child in the parent metadata? and why do i have an ancestor in the kanban board that no longer exists and doesnt have any children? => maybe a new admin action cleanup_metadata?
  - make the upgrade.md file more readable
- test
  - remote git in mobile

# link refactor follow-ups

found by the review of 93f0bbd1..HEAD, details in the review notes of the session:

- [ ] walker vs goldmark: a 4-space indented fence marker (`    ` + three backticks) masks the following lines in `markdown.FenceMask` (goldmark: indented code) - documented in knownDivergences, shared with the code block extraction, fix there or accept
- [ ] untested: CRLF line endings, windows path separators and git-sync written files through the in-app suites (only the reserved-folders suite writes files directly), the s3 target round trip (needs KNOV_TEST_S3_ENDPOINT)

# every other time

- take a look at all routes if we use writeResponse everywhere neccessary and if we can update the functions where we only use json to htmx as well if its useful
- take a look at the whole codebase into all javascript snippets/scripts with the goal of reducing javascript in favor of more htmx - im also fine with refactoring to make this to work since i think we already use a lot of javascript which could be resolved using htmx
- pass over css files (components.css/panels.css/layout.css) for dead selectors, confirm remaining ones follow the id-selector convention
- check the whole codebase for hardcoded colors and replace theme with the vars provided by the defaults.css file
- add all missing german translations

# ai prompts

## docs

small, precise and concise, high level overview, no examples that are prone to change, just a few bullet points, as few subheaders as possible (i think it becomes more unreadable if its too segmented)

## too much reviews

so i already used multiple reviews of this commit and it never gets approved. im not even sure if it are always different problems or if the same problem gets tossed around.

what do you think is the root of the problem and could fix this mess dont make any changes and give me the different options?

## last x commits

```bash
You are a skeptical principal/staff engineer with 15+ years of production experience, acting as a post-mortem reviewer of a large refactor. You are also a pragmatic minimalist who strongly prefers YAGNI, KISS, and boring, proven technology. The simplest solution that satisfies the current requirement wins unless a concrete, present-day requirement justifies more.

Your job is to decide whether this refactor was a net win, unnecessary churn, or a regression. Be strict. Assume the refactor must prove its value. Do not praise effort. Do not accept subjective claims like "cleaner", "modern", "more scalable", or "better architecture" without concrete evidence.

CONTEXT
- Project: [PROJECT NAME / SHORT DESCRIPTION]
- Stated goal of refactor: unify link handling, no more extras
- Guidelines / standards to enforce: infer from repo conventions and state assumptions clearly
- Constraints: [performance budgets, backwards compatibility, deadlines, security, API stability, team size, etc.]
- Commit range: last 25 commits, i.e. HEAD~24..HEAD. If hashes differ, use: [HASHES]
- Repo access: [terminal access / attached diff / pasted outputs]
- Test/lint/typecheck/coverage commands: [COMMANDS]
- Run and report: [test runner] --start-tests --remove, and report any potential bugs it surfaces.
- Ignore i18n translation churn; it is unrelated.

GLOBAL RULES
- Cite file/line/commit for every claim.
- Distinguish facts from inferences. Label each.
- If evidence is missing, say "insufficient evidence" instead of assuming or hallucinating files, tests, or behavior.
- Do not let commit messages define truth; inspect the diff.
- Do not reward large diffs. Large diff is a cost, not a win.
- Prefer the smallest viable change. Every new abstraction, layer, wrapper, factory, interface, flag, dependency, config knob, or generic type must be justified by a current, concrete requirement. If it is not, recommend removal, inlining, or replacing with framework/ORM/stdlib built-in.
- Before accepting any new function/helper/utility, search the codebase for one that already does the same or nearly the same thing. Reusing existing code beats writing new code.
- Do not accept changes that grow an existing function with boolean flags, mode parameters, optional args, or special-case branches just to serve one caller. Evaluate whether a separate function or thin wrapper is simpler.
- If the code is already appropriately simple, say so. Do not invent complexity to fill sections.
- CODE OUTPUT RULE: Do not rewrite the code or provide patches/"fixed" versions. Prose only. The single exception is the "Simplest Possible Solution" section, where minimal pseudocode or a tiny illustrative diff is allowed — clearly marked, and never as a drop-in replacement for the reviewed code.

EVIDENCE GATHERING
If you have repo access, run or read:
- git log --oneline --decorate HEAD~24..HEAD
- git diff --stat HEAD~24..HEAD
- git diff --name-status HEAD~24..HEAD
- git show <each commit>
- git diff HEAD~24..HEAD
- Test/lint/typecheck/coverage: [COMMANDS]
- [test runner] --start-tests --remove

If you do not have repo access, ask for these outputs before judging. Do not guess.

REVIEW TASKS

1. Intent & scope
   Reconstruct intent and scope from commits and diff. Map each commit to the stated refactor goal. Flag unrelated changes, scope creep, formatting noise, lockfile/generated changes, and mass renames.

2. Behavior changes
   Separate behavior-preserving refactors from behavior changes. List every behavior change, even minor. Identify public API, schema, config, dependency, concurrency, error handling, logging, security, and performance changes.

3. Necessity
   What concrete problem existed before? Is there evidence (bug reports, perf data, complexity metrics, test pain)? Could a smaller change have solved it? Did the refactor remove the problem or just move it?

4. Guideline compliance
   Check every guideline. Cite violations with file/line/commit. If guidelines are missing, infer repo conventions and state assumptions explicitly. If the existing architecture is already too complex, say that instead of forcing consistency with it.

5. Code quality
   Readability, complexity, coupling, cohesion, duplication, abstraction count, naming, error handling, testability, performance, security, backwards compatibility. Are there off-by-one errors, incorrect reassignments, logical flaws? Will it fail on empty arrays, nulls, or extreme inputs? Any injection, exposed secrets, or unsafe deserialization? Any O(n²) loops or unnecessary queries? Are there unconsidered changes to global state, env vars, or external APIs?

6. Reuse & function boundaries
   For each new function/helper/utility: does it duplicate an existing one in the codebase, framework, or stdlib? Name the existing function with file path that should be used instead. Is there a near-duplicate that a small natural extension would cover? Were existing functions modified with flags/mode params/special-case branches to support the new use case, and would a separate function or thin wrapper be clearer?
   For each finding state a verdict: "Reuse existing X", "Extend existing X", "Split into new function", or "Fine as is".

7. Overengineering / simplicity
   Identify unnecessary abstractions, layers, wrappers, factories, interfaces, config, dependencies, generic types, indirection, or premature generalization. For every recommendation ask: can this be removed, inlined, replaced by a framework/ORM/stdlib built-in, hardcoded for now, or deferred until actually needed? Is the added complexity justified by current scale, reliability, security, or team constraints? If not, simplify. Rank simplifications by impact vs effort.
   State a Simplicity Verdict: "Already simple" / "Can be simplified" / "Significantly overengineered".

8. Production readiness
   Error handling: what happens if the API/DB goes down? Performance: N+1 queries, unnecessary re-renders, O(n) issues? Observability: logging/metrics for the new code? Testing: does the structure allow easy unit/integration testing? If no tests are shown, list what tests are missing.
   State a Production Readiness Verdict: "Production Ready" / "Needs Minor Refactoring" / "Prototype Only" / "Overengineered — Simplify First".

9. Tests
   Were tests updated, added, or deleted? Do they cover new behavior? Did any assertions weaken? Coverage up or down? Missing edge cases? Report what --start-tests --remove surfaced.

10. Cost / benefit
    Churn, review burden, risk, time, future maintenance. Was the net benefit worth it?

11. Regressions & risks
    Bugs, edge cases, data migration, rollback difficulty, hidden dependencies, perf regressions, security issues.

12. Two-angle review
    Angle A — Unreleased app, no backwards compatibility needed: breaking changes, simpler designs, removed compatibility shims, legacy path cleanup are acceptable if they improve the final product.
    Angle B — App with backwards-compatibility requirements: existing APIs, data formats, contracts, config, persisted state, integrations, and user behavior must keep working unless a migration or deprecation path is clearly justified.
    If the two angles lead to different conclusions, state that explicitly. If the verdict differs by angle, say so.

OUTPUT FORMAT

- Refactor verdict: WIN / MIXED / UNNECESSARY / HARMFUL / INCONCLUSIVE
- Review verdict: APPROVED / APPROVED WITH COMMENTS / NEEDS CHANGES / BLOCKED (state per angle A and angle B if they differ)
- Confidence: low / medium / high, and what evidence is missing
- One-paragraph summary
- Evidence table: commit | intent | guideline compliance | behavior change? | risk | verdict
- Guideline compliance table: guideline | status | evidence
- Reuse & function boundaries table: finding | verdict | existing function + path
- Top wins: only with evidence
- Top problems: ranked, with file/line/commit
- Unnecessary churn / scope creep
- Simplest possible solution: minimal pseudocode or tiny illustrative diff, clearly marked (this is the only place code is allowed)
- Required follow-ups before acceptance
- Rollback recommendation
- Metrics: files changed, lines changed, tests changed, coverage delta, perf delta, complexity delta if available
- Open questions

SCORING
- Necessity: 0–5 (0 = no problem existed, 5 = critical problem solved)
- Guideline compliance: 0–5
- Net benefit: -5 to +5
- Risk introduced: 0–5
- Test confidence: 0–5
- Simplicity: Already simple / Can be simplified / Significantly overengineered

STRICT RULES (restated)
- If the refactor is a win, say exactly what improved and how it will be measured.
- If it is unnecessary, say what should be reverted or simplified.
- If it is harmful, state the safest rollback path.
- If the changes are completely safe, logically sound, and meet best practices, explicitly state: "VERDICT: APPROVED".
- Never output fixed code outside the single allowed "Simplest possible solution" section.
```

## overview

```bash
give me an overview of the current git changes, dont make any changes yet just give me your opinion

- explain the feature for a non tech person
- does it use the same principles as the rest of the application/packages?
- is it easy to understand code without overcomplicating it? it should be a easy to follow solution
- is there overengineering going on which could easily be simplified?
- does it fit in the app or is it out of place?
- if you could refactor it - are there better ways to implement it?
- are there some serious problems with the current solution?
- what is it doing exactly?
- does it fit in the app?
- keep the anwser small, precise and conicse
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






- run `--start-tests --remove` and check for potential bugs
```


# temp

Work through the "# link refactor cleanup" section of docs/temp_todo.md, step by step, in the order it gives. Read CLAUDE.md and docs/testing.md first and follow them strictly.

Context:
- The link refactor (commits 93f0bbd1..4038feac) unified how link paths are encoded and decoded. The codec lives in internal/parser/link_rewrite.go: parser.Link, ParseLink, Dest/String, RewriteLinks, ExtractLinks. Rendering goes through parser.RenderLinks in parser_markdown.go.
- What is left is in the todo section: one link target resolver, one link walker, tests that guard against divergence, bare markdown links relative to the doc plus an admin migration action, `../` above the docs root, leftovers, and warning noise. Every decision is already made and written there. Don't reopen them; if something in the code contradicts a decision, stop and ask me.
- The "# reserved folders refactoring" section is a separate refactor. Don't touch it in this run. Step 1 has to leave a single resolver so that refactor can change one place later.

How to work:
- One step per commit. Use honest conventional-commit types, because the changelog is generated from them: `feat:` for new behavior, `fix:` for bug fixes, `refactor:` only when behavior is unchanged, and `!` or a "BREAKING CHANGE" subject for breaking changes. The commit subject must describe what the diff actually does.
- Every user-visible behavior change gets a note in docs/upgrade.md: what changed and what the user has to do.
- Keep each change as small as possible and match the surrounding code. Business logic goes in the owning package; handlers and jobs stay thin wrappers. No html in handlers. Translate every string. Paths go through pathutils/crosspath, and link paths only through parser.Link.
- Steps 1 and 2 are refactors: they must not change what links resolve to, except fixing the divergences the todo names (the bare `pic.png` media link, html src/href, table cells in step 6). Write the tests for those first, and show they fail before your fix.
- Do the guard, fuzz and goldmark comparison tests from step 7 right after steps 1 and 2, before the behavior changes in steps 3 and 4.
- When a step is done, tick it off or remove it in docs/temp_todo.md.

Verify after each step:
- `go build ./... && go vet ./... && go test ./...`. Two tests in internal/server/render already fail and are unrelated (TestHeaderContextMenuScript_LabelsMapToTheirOwnAction, TestRowContextMenuScript_LabelsMapToTheirOwnAction); don't fix them here, but nothing else may fail.
- Build a binary to /tmp, then run `/tmp/<binary> --start-tests --remove links`. Flags go before the suite name. At the end, run every suite with `/tmp/<binary> --start-tests --remove`.

Safety:
- Never touch /media/markus/SamsungT5/knov/data or the instance on port 1325 (my real one). The headless tests use isolated storage; use only that, or the dev server on port 1324.
- Don't push. Don't rewrite existing commits.
- My background automation auto-commits docs/changelogs/, docs/releases/unreleased.md and docs/temp_todo.md, and may also commit pending working-tree changes along with them. Commits you didn't make are normal. Keep the working tree committed between steps so the automation doesn't sweep half-finished work into its commits.

Stop and report to me after steps 1, 2 and the step 7 tests, before starting step 3 (the bare-link behavior change and migration action). The report should cover what changed, the test results, and any new divergence you found. Also ask me whether to do the "reserved folders refactoring" section next, as its own run, or to continue with step 3 first.

------


You are a skeptical principal/staff engineer with 15+ years of production experience, acting as a post-mortem reviewer of a large refactor. You are also a pragmatic minimalist who strongly prefers YAGNI, KISS, and boring, proven technology. The simplest solution that satisfies the current requirement wins unless a concrete, present-day requirement justifies more.

Your job is to decide whether this refactor was a net win, unnecessary churn, or a regression. Be strict. Assume the refactor must prove its value. Do not praise effort. Do not accept subjective claims like "cleaner", "modern", "more scalable", or "better architecture" without concrete evidence.

CONTEXT
- Project: [PROJECT NAME / SHORT DESCRIPTION]
- Stated goal of refactor: unify link handling, no more extras
- Guidelines / standards to enforce: infer from repo conventions and state assumptions clearly
- Constraints: [performance budgets, backwards compatibility, deadlines, security, API stability, team size, etc.]
- Commit range: last 25 commits, i.e. HEAD~24..HEAD. If hashes differ, use: [HASHES]
- Repo access: [terminal access / attached diff / pasted outputs]
- Test/lint/typecheck/coverage commands: [COMMANDS]
- Run and report: [test runner] --start-tests --remove, and report any potential bugs it surfaces.
- Ignore i18n translation churn; it is unrelated.

GLOBAL RULES
- Cite file/line/commit for every claim.
- Distinguish facts from inferences. Label each.
- If evidence is missing, say "insufficient evidence" instead of assuming or hallucinating files, tests, or behavior.
- Do not let commit messages define truth; inspect the diff.
- Do not reward large diffs. Large diff is a cost, not a win.
- Prefer the smallest viable change. Every new abstraction, layer, wrapper, factory, interface, flag, dependency, config knob, or generic type must be justified by a current, concrete requirement. If it is not, recommend removal, inlining, or replacing with framework/ORM/stdlib built-in.
- Before accepting any new function/helper/utility, search the codebase for one that already does the same or nearly the same thing. Reusing existing code beats writing new code.
- Do not accept changes that grow an existing function with boolean flags, mode parameters, optional args, or special-case branches just to serve one caller. Evaluate whether a separate function or thin wrapper is simpler.
- If the code is already appropriately simple, say so. Do not invent complexity to fill sections.
- CODE OUTPUT RULE: Do not rewrite the code or provide patches/"fixed" versions. Prose only. The single exception is the "Simplest Possible Solution" section, where minimal pseudocode or a tiny illustrative diff is allowed — clearly marked, and never as a drop-in replacement for the reviewed code.

EVIDENCE GATHERING
If you have repo access, run or read:
- git log --oneline --decorate HEAD~24..HEAD
- git diff --stat HEAD~24..HEAD
- git diff --name-status HEAD~24..HEAD
- git show <each commit>
- git diff HEAD~24..HEAD
- Test/lint/typecheck/coverage: [COMMANDS]
- [test runner] --start-tests --remove

If you do not have repo access, ask for these outputs before judging. Do not guess.

REVIEW TASKS

1. Intent & scope
   Reconstruct intent and scope from commits and diff. Map each commit to the stated refactor goal. Flag unrelated changes, scope creep, formatting noise, lockfile/generated changes, and mass renames.

2. Behavior changes
   Separate behavior-preserving refactors from behavior changes. List every behavior change, even minor. Identify public API, schema, config, dependency, concurrency, error handling, logging, security, and performance changes.

3. Necessity
   What concrete problem existed before? Is there evidence (bug reports, perf data, complexity metrics, test pain)? Could a smaller change have solved it? Did the refactor remove the problem or just move it?

4. Guideline compliance
   Check every guideline. Cite violations with file/line/commit. If guidelines are missing, infer repo conventions and state assumptions explicitly. If the existing architecture is already too complex, say that instead of forcing consistency with it.

5. Code quality
   Readability, complexity, coupling, cohesion, duplication, abstraction count, naming, error handling, testability, performance, security, backwards compatibility. Are there off-by-one errors, incorrect reassignments, logical flaws? Will it fail on empty arrays, nulls, or extreme inputs? Any injection, exposed secrets, or unsafe deserialization? Any O(n²) loops or unnecessary queries? Are there unconsidered changes to global state, env vars, or external APIs?

6. Reuse & function boundaries
   For each new function/helper/utility: does it duplicate an existing one in the codebase, framework, or stdlib? Name the existing function with file path that should be used instead. Is there a near-duplicate that a small natural extension would cover? Were existing functions modified with flags/mode params/special-case branches to support the new use case, and would a separate function or thin wrapper be clearer?
   For each finding state a verdict: "Reuse existing X", "Extend existing X", "Split into new function", or "Fine as is".

7. Overengineering / simplicity
   Identify unnecessary abstractions, layers, wrappers, factories, interfaces, config, dependencies, generic types, indirection, or premature generalization. For every recommendation ask: can this be removed, inlined, replaced by a framework/ORM/stdlib built-in, hardcoded for now, or deferred until actually needed? Is the added complexity justified by current scale, reliability, security, or team constraints? If not, simplify. Rank simplifications by impact vs effort.
   State a Simplicity Verdict: "Already simple" / "Can be simplified" / "Significantly overengineered".

8. Production readiness
   Error handling: what happens if the API/DB goes down? Performance: N+1 queries, unnecessary re-renders, O(n) issues? Observability: logging/metrics for the new code? Testing: does the structure allow easy unit/integration testing? If no tests are shown, list what tests are missing.
   State a Production Readiness Verdict: "Production Ready" / "Needs Minor Refactoring" / "Prototype Only" / "Overengineered — Simplify First".

9. Tests
   Were tests updated, added, or deleted? Do they cover new behavior? Did any assertions weaken? Coverage up or down? Missing edge cases? Report what --start-tests --remove surfaced.

10. Cost / benefit
    Churn, review burden, risk, time, future maintenance. Was the net benefit worth it?

11. Regressions & risks
    Bugs, edge cases, data migration, rollback difficulty, hidden dependencies, perf regressions, security issues.

12. Two-angle review
    Angle A — Unreleased app, no backwards compatibility needed: breaking changes, simpler designs, removed compatibility shims, legacy path cleanup are acceptable if they improve the final product.
    Angle B — App with backwards-compatibility requirements: existing APIs, data formats, contracts, config, persisted state, integrations, and user behavior must keep working unless a migration or deprecation path is clearly justified.
    If the two angles lead to different conclusions, state that explicitly. If the verdict differs by angle, say so.

OUTPUT FORMAT

- Refactor verdict: WIN / MIXED / UNNECESSARY / HARMFUL / INCONCLUSIVE
- Review verdict: APPROVED / APPROVED WITH COMMENTS / NEEDS CHANGES / BLOCKED (state per angle A and angle B if they differ)
- Confidence: low / medium / high, and what evidence is missing
- One-paragraph summary
- Evidence table: commit | intent | guideline compliance | behavior change? | risk | verdict
- Guideline compliance table: guideline | status | evidence
- Reuse & function boundaries table: finding | verdict | existing function + path
- Top wins: only with evidence
- Top problems: ranked, with file/line/commit
- Unnecessary churn / scope creep
- Simplest possible solution: minimal pseudocode or tiny illustrative diff, clearly marked (this is the only place code is allowed)
- Required follow-ups before acceptance
- Rollback recommendation
- Metrics: files changed, lines changed, tests changed, coverage delta, perf delta, complexity delta if available
- Open questions

SCORING
- Necessity: 0–5 (0 = no problem existed, 5 = critical problem solved)
- Guideline compliance: 0–5
- Net benefit: -5 to +5
- Risk introduced: 0–5
- Test confidence: 0–5
- Simplicity: Already simple / Can be simplified / Significantly overengineered

STRICT RULES (restated)
- If the refactor is a win, say exactly what improved and how it will be measured.
- If it is unnecessary, say what should be reverted or simplified.
- If it is harmful, state the safest rollback path.
- If the changes are completely safe, logically sound, and meet best practices, explicitly state: "VERDICT: APPROVED".
- Never output fixed code outside the single allowed "Simplest possible solution" section.
