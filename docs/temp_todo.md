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

# reserved folders refactoring

goal: docs paths are unambiguous, so no top-level docs folder name (docs, media, files) has to be reserved. rule: a docs path entering pathutils always carries an explicit "docs/" prefix - user input, /files/<rel> urls and listings are docs-relative and taken literally, the prefix guessing of pathutils.parsePath only stays for links and old metadata. own run, one commit per step, `--start-tests --remove` at the end.

- [x] R1 path model: pathutils.DocsPath(rel) - a literal docs-relative path as "docs/" path (no prefix stripping), go tests that docs/media/x.md, docs/docs/x.md and docs/files/x.md resolve to themselves through ToDocsPath / ToFullPath / ToWithPrefix / ToRelative
- [ ] R2 listing: contentStorage.ListFiles callers and files.pathsToFiles give docs files a "docs/" File.Path, fix the File.Path consumers that expect it unprefixed (ViewURL, display, filters)
- [ ] R3 url routes: /files/, /files/edit/, /files/edittable/, /files/history/, pathutils.FileFromURL (upload context_path, viewedFile / HX-Current-URL) and the api path routes reading r.URL.Path (content, rename, move-folder, delete, delete-folder, versions, versions/diff, versions/restore) read their rel literally (DocsPath)
- [ ] R4 query / form params: every handler path param (filepath, path, folder, prefillpath, parents, ... ~66) and its senders (templates, js, render) - per param decide docs-relative (literal) or metadata path (explicit docs/ or media/ prefix, e.g. where media files are accepted too) and convert at the handler boundary
- [ ] R5 links: one place since the link cleanup - parser.LinkTarget / ResolveLinkPath / utils.NormalizeLinkPath. a /files/<rel> link is literal; decide and document how a docs file in docs/media/ is linked ([[media/x]] and bare media/ stay media), help page, links suite case for docs/media/, docs/docs/, docs/files/
- [ ] R6 other stored paths: metadata parents, kanban folders, filter criteria folder values, Auto-Create Tags folders, configeditor ids / PairedPath (filter, tracker)
- [ ] R7 migration: metadata of docs files under docs/docs/, docs/media/, docs/files/ (their old keys collide with real media / docs keys), filter / tracker configStorage ids starting with docs/, media/ or files/ (configeditor.CleanID)
- [ ] R8 remove the workaround: configmanager.ReservedDocsFolders, the reserved check in pathutils.CheckTarget, ErrReservedPath (+ server newPathMessage / handleMoveError), the reserved checks in configeditor.CleanID and validateKanbanFolder (+ the hint in the Auto-Create Tags setting Desc), turn the reserved cases of TestCheckNewDocsPath into "these paths resolve correctly" tests. keep the filename policy (CheckTarget / ErrInvalidName)
- [ ] R9 suite: files in docs/docs/, docs/media/, docs/files/ created by the app and written directly (git sync) - list, view, edit, metadata, links, rename, move, delete
- [ ] R10 upgrade note, run every suite (`--start-tests --remove`)

context (from the review, decisions included):

- todo: make docs paths unambiguous so no top-level docs folder name has to be reserved
  - problem: pathutils.parsePath guesses the type from free text - a leading "files/" is stripped, "media/..." is read as a media file and "docs/..." as a docs file. a docs file at data/docs/media/x.md (or docs/docs/..., docs/files/...) therefore can't be resolved back to itself
    - current workaround (keep until this is done): configmanager.ReservedDocsFolders + pathutils.ErrReservedPath (checked first in pathutils.CheckTarget, before the filename policy) reject creating/moving files there (handlers, files.MoveFileNoRefresh/MoveFolder, configeditor.CleanID, validateKanbanFolder), existing files are exempt via os.Stat
    - the workaround only covers files created by the app - files that arrive via git pull/sync or are copied into the filesystem are still broken:
      - contentStorage.ListFiles + files.pathsToFiles list them as "media/x.md" without a docs/ prefix
      - File.ViewURL points to /files/x.md => docs/x.md (404 or the wrong file)
      - their metadata key "media/x.md" is the same key as a real media file data/media/x.md, so the two overwrite each other's metadata
  - fix:
    - docs paths always carry an explicit "docs/" prefix internally (listing, metadata keys, File.Path)
    - user input (form paths, rename/move targets) and /files/<rel> URLs are treated as literal docs-relative paths - no prefix stripping (e.g. a separate docs-rel => full path function next to ToDocsPath). this includes pathutils.FileFromURL (upload context_path, viewedFile / HX-Current-URL) and the path routes that read r.URL.Path (rename, delete, move-folder, metadata rebuild, versions) - today their rel goes through ToDocsPath, so /files/media/x.md resolves to the media folder
    - do this after "link refactor cleanup" steps 1-2 (one link target resolver, one link walker) - then the link part below is a change in that one resolver instead of three places
    - keep the prefix guessing only where it's really needed (old metadata) - for links in content that's one place since the link refactor: utils.NormalizeLinkPath (after parser.ParseLink decoded the path), used by metadata, rename (renameLinkFunc) and FindBrokenLinks; the renderer has its own branch in parser.appLinkDest (media/ and /media/ -> ToMediaURL, the rest -> docLinkDest) and media relocate its candidates in relocateIndex.resolve - change all three together, the links suite (internal/test/linkstest) covers them
    - catch: a link like [[media/x.md]] still resolves to media, so docs files in docs/media/ need a "docs/media/x.md" link - decide and document this (in NormalizeLinkPath + appLinkDest), add a docs/media/ case to the links suite
  - migration: metadata keys of docs files that currently collide with media keys, and filter/tracker configStorage ids starting with docs/, media/ or files/ (see configeditor.CleanID)
  - afterwards delete: ReservedDocsFolders, the reserved check in CheckTarget, ErrReservedPath (+ its case in server newPathMessage / handleMoveError), the reserved check in configeditor.CleanID and validateKanbanFolder (+ the hint in the Auto-Create Tags setting Desc), and turn the reserved cases of TestCheckNewDocsPath (pathutils_test.go) into "these paths now resolve correctly" tests
  - keep: the filename policy from the link refactor (CheckTarget / ErrInvalidName, writeNewPathError / newPathMessage)
  - touches many ToDocsPath/ToWithPrefix/ToRelative callers - do it as its own refactor, run `--start-tests --remove` afterwards

# link refactor cleanup

done (93f0bbd1..4038feac review follow-ups, details in the commits and docs/upgrade.md):

- [x] 1 one link target resolver - parser.LinkTarget (`fix:` ae7de291)
- [x] 2 one link walker - parser.walkLinks (`refactor:` 6af90df0)
- [x] 7 guard tests - FuzzLinkCodec / FuzzRewriteLinksIdentity, TestScannerMatchesGoldmark, TestNoHandRolledLinkHandling (`test:` ae2eb2c4)
- [x] 3 bare markdown / html links read from the doc's folder + admin "Bare Links Migration" - a bare `media/...` link stays media, html follows markdown (`feat!:` 2ace0341)
- [x] 4 `../` above the docs root listed and repaired in "Repair Broken Links", content scan, no metadata field (`feat:` f6a32955)
- [x] 5 no warning noise for folder links / files moved along in folder moves (`fix:` f7095680)
- [x] 6 table cells through RenderLinks, fileHistoryURL template func, dead dokuwiki branch and media select list removed (`fix!:` d74e1dd0)
- [x] 7 rest - guard allow-list lowered after 6
- [x] 8 every suite run: 182 passed, 1 skipped (s3)
- decided, nothing to do: keep the filename policy (pathutils.CheckTarget / ErrInvalidName), keep the title fallback removal

follow-ups (found by the tests, not fixed yet - decide):

- [ ] codec: ascii control characters besides tab / line breaks aren't percent-encoded in link paths - a "\f" ends a markdown destination, so the link is lost (FuzzLinkCodec's fuzzPath skips them)
- [ ] codec + walker: "`" isn't encoded - two of them in a path are a code span for maskCode, the walker skips the link while goldmark reads them as part of the destination (fuzzPath skips them, a knownDivergences case)
- [ ] walker vs goldmark (knownDivergences in TestScannerMatchesGoldmark): a destination, title or reference definition destination on the next line (goldmark: link, walker: none), an escaped "\[" before "](dest)", a link in an indented code block or an html comment (walker: link, goldmark: none)
- [ ] images: markdown images still render through renderImage / resolveMediaPath, not LinkTarget - a bare `![x](pic.png)` that exists only in the docs folder renders /media/... (404) while link metadata reads docs/..., an image named with a trailing space ("trail.png ") renders nothing
- [ ] images in interactive table cells render with plain goldmark (RenderInlineMarkdown), not renderImage - `![x](media/pic.png)` gets a src relative to the page
- [ ] other hand-rolled link readers / writers (allowed in guardRules): pdfexport/images.go zoneImageLinkRe (header/footer zone image template), dokuwikiconverter/converter_process.go builds an unencoded `[url](url)` /browse/folders link, server/render/render_editor_codemirror.go strips link syntax to text with its own regexes
- [ ] markdown links with a "|" in the path break a table cell (the codec doesn't escape "|" for markdown, GFM needs `\|` in cells) - the links suite table case skips them
- [ ] flaky: dashboard suite widget-filter-data failed once in a full `--start-tests --remove` run, passed in the rerun and 3 runs alone
- [ ] unrelated, already failing: internal/server/render TestHeaderContextMenuScript_LabelsMapToTheirOwnAction, TestRowContextMenuScript_LabelsMapToTheirOwnAction (table editor)

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
