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

found by the post-mortem review of 93f0bbd1..HEAD. each item has a failing input, add a test for it:

- [x] scanner: links in tab-indented nested lists are missed - `- a\n\n\t- b\n\n\t\t- c [x](c.md)` (walker `[]`, goldmark `[c.md]`). `listMarkerRe` in `internal/parser/link_rewrite.go` only matches spaces (`^( *)`) and `indentedCodeMask` keeps one `listContent` instead of a stack, so a tab-indented or outdented item reads as indented code. expand tabs to columns, pop to the ancestor item on an outdent. add the cases to `parser_goldmark_diff_test.go`, plus a bounded structural fuzz (nested lists with spaces/tabs/blank lines/`>`) comparing walker and goldmark
- [x] scanner, smaller divergences found by fuzz: a tab after `>` (`>\t[x](a.md)`, walker misses it), a list marker with 5+ spaces (`1.     [x](a.md)` is code in goldmark), an unclosed `<!--` after a list marker or `>`, a numeric entity in a path (`[x](&#120;.md)`: the walker cuts at the `#`), an orphan `](` swallowing the next link (`]([k](a.md))`). fix or add to knownDivergences with the reason - done: tab after `>`, 5+ spaces after a marker, numeric entity, orphan `](` fixed; the block comment inside a list item or quote is in knownDivergences (it ends with its container, the scanner only knows ones starting a line)
- [x] scanner: a 4-space indented fence marker masks the following lines in `markdown.FenceMask` (goldmark: indented code), already in knownDivergences - fix there or accept (a tab-indented one diverges the same way) - accepted: FenceMask has no list context and an indented fence is common inside list items (`- a` + 4 spaces + fence), so skipping 4+ spaces would break those; both forms stay in knownDivergences
- [x] bare, `./` and `/x` links must resolve to the docs file when it exists, media only as fallback (`linkTarget` in `link_rewrite.go` checks `media/` first, so `[x](p.pdf)` in `sub/n.md` opens `media/sub/p.pdf` although `docs/sub/p.pdf` exists). change the order, update the `LinkTarget` comment, `help.gohtml` and the upgrade note, add a test for both files existing
- [x] `MetaDataPurgeStale` (full rebuild) deletes the legacy keys the manual reserved folders migration would move (`media/x.md` for `docs/media/x.md` when no media file exists). skip a key while a docs file in `docs/docs`, `docs/media` or `docs/files` maps to it, add a test (migrate after a full rebuild keeps the record)
- [x] wiki rename adds a `docs/` prefix (`[[sub/x]]` -> `[[docs/sub/y]]`, `renameLinkFunc` in `metadata_links.go`). keep it only when the path starts with `media/` or `docs/`, like `parser.DocsWikiPath`
- [x] broken links repair entries are `source|target|suggested` split on `|` (`manualjob.go:308`): a legacy file with `|` in its name is mis-split and a malformed entry is dropped without counting as skipped. send them as json like `relative-links/migrate` and count a malformed one as skipped (status stays 200 with the repaired/skipped counts)
- [x] `/files/` is read in three places (`utils.NormalizeLinkPath`, `renderImage` in `parser_markdown.go`, `rebuildLinkTarget`) and written as `"/files/" + TrimPrefix(p, "docs/")` six times (`rebuildLinkTarget` x4, `filter.go`, `FileLinkDest`): use `NormalizeLinkPath` for reading and one helper for writing - done: one writer `parser.FilesLinkPath`; `renderImage` only tests the prefix of an already rendered url, not a path read, left as is
- [ ] js still handles link paths by hand: `static/wiki-autocomplete.js` (`FILES_PREFIX` strip, `decodeURIComponent`, `docs/` strip), `themes/builtin/js/panel-file.js` (`"docs/" + filepath`), `themes/builtin/mediaview.gohtml` (`printf "media/%s"`). let the server return the values (like `data-link`) and extend `parser_guard_test.go` to `decodeURIComponent`
- [ ] `handleAPIGetFileContent` answers 500 for a missing file (404), the new migrate/repair handlers send `err.Error()` untranslated, `handleAPIGetAncestorsInFolder` builds html in the handler (move to render)
- [ ] `TestMoveRedirectsOnlyViewedFile` fails with `-count=2` (409 already exists, state kept between runs), the in-app tests share the fixed `/tmp/knov_temp_test` so two runs at once corrupt each other (`SQLITE_BUSY`): use a unique temp dir per run
- [ ] untested: git-sync written files through the in-app suites, windows paths and html src/href in table cells, the s3 target round trip (needs KNOV_TEST_S3_ENDPOINT)

# typed paths refactor

the docs-relative path, the metadata path (`docs/...` / `media/...`) and the full path are all plain `string`, and `ToRelative` / `ToWithPrefix` / `ToDocsPath` guess the kind from a prefix (about 286 call sites). `ToWithPrefix(ToRelative(p))` is lossy for `docs/docs/` and `docs/media/` files, so every new call can pick the wrong file again. goal: make the wrong call not compile. do it in this order, one commit per step:

- [ ] agreement test first: link forms x file locations (`docs/x.md`, `docs/docs/x.md`, `docs/media/x.md`, `docs/files/x.md`, `media/x.png`, the special-char names) x every consumer (metadata, render, rename, move, media relocation, filter index, book, kanban, dashboard, search). a new consumer has to be registered there. it fails for the three known bugs below until they are fixed
- [ ] add `pathutils.MetaPath` (always `docs/...` or `media/...`) and `pathutils.DocsRel`, built only by constructors (`DocsPath(rel)`, `MediaPath(rel)`, `ParseMeta(s)` for input that must already be a metadata path, like `metaPathParam`). guessing stays only where user input enters (form values, typed links, settings)
- [ ] convert package by package, boundaries first: `files.OnFileMoved`, metadata keys, filter criteria (`child-of`, `parent-of`, `ancestor-of`), dashboard, kanban order and events, search index keys, `File.Path`
- [ ] known bugs the conversion has to fix (keep the regression test for each):
  - `files.OnFileMoved` gets docs-relative paths (`physical.go:90,276`) and `dashboard.PatchFilePathForMove` runs `ToRelative` on them again (`dashboard.go:191-196`, not idempotent: `media/x.md` -> `x.md`): moving `docs/media/x.md` patches a widget of `docs/x.md` and writes `y.md` for `docs/media/y.md`
  - `filter.GenerateFilterIndex` writes `/docs/x.md` for `docs/docs/x.md`, which reads as `docs/x.md`: write `/files/` + the path without `docs/` for every docs file (media files keep `/media/`)
  - kanban ancestor select: `handleAPIGetAncestorsInFolder` uses `ToRelative(a)` as option value and `filter.go` compares `ToWithPrefix(value)`, so an epic in `docs/media/` or `docs/docs/` never matches: use the metadata path as value, compare exactly
- [ ] make `parsePath` private and remove `ToRelative` / `ToWithPrefix` / `ToDocsPath` / `ToFullPath` on arbitrary strings. until then add them to `parser_guard_test.go` (also `ToWithPrefix(ToRelative(`) with an allow-list that only shrinks
- [ ] rule in `CLAUDE.md`: a variable carries its kind in the name (`metaPath`, `docsRel`), no function takes a bare "path" for docs files

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
- For any claim that the refactor is “easy to follow” or “hard to diverge from,” require evidence: number of canonical entrypoints, old paths removed or deprecated, tests/lint/type/CI checks that enforce the invariant, and docs/ADR. Otherwise label it “insufficient evidence.”
- If the new pattern still allows the old pattern to compile, pass tests, or be copied safely, assume it will diverge unless explicitly guarded.

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

13. Drift resistance & convergence
    Is the result easy to follow? Cite the canonical path(s) with file/line/commit.
    Is it easy to diverge again? List every remaining alternate path, compatibility shim, flag, optional arg, duplicate helper, old call site, or undocumented pattern that can reintroduce the old behavior.
    If easy to diverge, name the concrete divergence trigger: copy-paste, missing test, multiple entrypoints, no lint/type enforcement, unclear boundary, stale docs, etc.
    What is the smallest prevention? Prefer: delete old path, reduce to one public entrypoint, add one focused test, add one lint/type rule, add CI check, add short ADR/comment. Do not add abstraction unless current requirement demands it.
    Is the refactor’s resulting convention strict enough to prevent recurrence? If not, state exactly what must be stricter.
    Is this review prompt strict enough? If not, state exactly which rule to add, tighten, or relax.

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
- Drift resistance verdict: Easy to follow / Guarded / Easy to diverge / Insufficient evidence
- Divergence risks: ranked list with file/line/commit
- Prevention required before acceptance: concrete minimal guardrails, or “none”
- Prompt strictness verdict: Adequate / Should be more restrictive / Should be less restrictive
- Recommended prompt change: exact rule to add, remove, or tighten, or “none”

SCORING
- Necessity: 0–5 (0 = no problem existed, 5 = critical problem solved)
- Guideline compliance: 0–5
- Net benefit: -5 to +5
- Risk introduced: 0–5
- Test confidence: 0–5
- Simplicity: Already simple / Can be simplified / Significantly overengineered
- Drift resistance: 0–5
  `0` = old paths remain and nothing enforces the new pattern
  `5` = single canonical path enforced by tests/lint/types/CI, old path removed or unreachable

STRICT RULES (restated)
- If the refactor is a win, say exactly what improved and how it will be measured.
- If it is unnecessary, say what should be reverted or simplified.
- If it is harmful, state the safest rollback path.
- If the changes are completely safe, logically sound, and meet best practices, explicitly state: "VERDICT: APPROVED".
- Never output fixed code outside the single allowed "Simplest possible solution" section.
- If easy to diverge, do not approve. Require removal of old paths or a concrete enforcement mechanism first.
- If the prompt is too loose, state the exact additional rule. If too strict for the evidence, state exactly which rule to relax.
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
