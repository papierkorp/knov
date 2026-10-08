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

follow-ups from the review of the link refactor (parser.Link codec, 93f0bbd1..4038feac). the codec (decode once / encode once) is done - what is left is that "link -> target file" is still resolved in several places, plus a few leftovers. order matters: do 1 and 2 first, the rest builds on them. the "reserved folders refactoring" above is a separate refactor (path model + metadata migration) that depends on 1 - keep them as separate commits/runs, don't mix the two migrations
- work order:
  - 1 (one resolver), 2 (one walker), the tests of 7 - then stop and report to the user
  - then start the "reserved folders refactoring" section above as its own run (it needs the single resolver from 1, its link part is a change in that one place) - before or after 3, the user decides when reporting
  - 3, 4, 5, 6, the rest of 7, 8

- decided, nothing to do:
  - keep the filename policy (pathutils.CheckTarget / ErrInvalidName)
  - keep the title fallback removal (titles only from stored metadata, files/metadata_cache.go)
- 1. one link target resolver - done: parser.LinkTarget (renderer, ExtractLinks, rename, relinkMovedDoc, FindBrokenLinks, relocate). left: markdown images still render through renderImage/resolveMediaPath (see report)
  - problem: the link path -> target file mapping is put together by hand at ~9 call sites, each combining pathutils.ResolveRelativeLink + utils.NormalizeLinkPath / WithDefaultLinkExt + files.resolveMediaLink slightly differently: parser_markdown.go (wikiLinkMarkdown, processMarkdownLink, processRefDefs, docLinkDest), book.resolveRelativeLinks, files/metadata_links.go (MetaDataLinksRebuild, updateUsedLinks, relinkMovedDoc, renameLinkFunc, FindBrokenLinks), files/media_relocate.go relocateIndex.resolve
  - existing divergence: a bare `[x](pic.png)` to the media file media/pic.png - link metadata says media/pic.png (resolveMediaLink stats the media folder), the renderer links /files/pic.png (404, /files/ has no media fallback) - so it's neither shown in the broken links scan nor working
  - fix: one function (e.g. parser.LinkTarget(docPath string, l parser.Link) string -> metadata path "docs/a.md" / "media/x.png" / "docs/sub/" for folders) used by all of them; the renderer builds its url from that result (appLinkDest) instead of its own branches
  - add a links suite / go test case: for every specialchars name and link kind, the rendered href and the link metadata point at the same file
  - include html src/href: a bare `<img src="pic.png">` in sub/n.md has three meanings today - link metadata reads it from the docs root (NormalizeLinkPath), media relocate from the doc's folder (relocateIndex.resolve default case), and the renderer leaves it untouched (RenderLinks has no LinkHTML handling), so the browser resolves it against the page url /files/sub/n.md. the resolver decides once and RenderLinks rewrites html src/href to that url like markdown links
- 2. one link walker - done: parser.walkLinks (masked once), RewriteLinks / ExtractLinks / RenderLinks use it
  - problem: RewriteLinks (rewriteLinkRe, per non-code line part) and RenderLinks (processMdLinkRe on the whole masked content, then processRefDefs as a second replaceOutsideCode pass = second maskCode) are two scanners sharing sub-patterns - they can drift
  - fix: one walker (masked once) that yields each link (parser.Link, its span, its "[text" / "![alt" opening, kind incl. ref defs and html attrs) and lets the caller replace it; RewriteLinks, ExtractLinks and RenderLinks become thin users of it
- 3. bare links relative to the doc (decision 2B) + admin migration action - done: parser.IsBareLink / ResolveLinkPath / DocsRootLinkTarget, files.ScanRelativeLinks / MigrateRelativeLinks, admin "Bare Links Migration". decided when doing it: a bare `media/...` link stays media (upload and media autocomplete insert it), html src/href follow markdown
  - do after 1 (then it is a change in one place)
  - change: a bare markdown link (`[x](a.md)` in sub/n.md) is read from the doc's folder like `./a.md` (CommonMark / GitHub behaviour), a leading "/" (`[x](/a.md)`) or `/files/` stays docs-root
  - decided: wikilinks stay docs-root (`[[a]]` in sub/n.md -> a.md) - wikilinks are page names like in dokuwiki / mediawiki / wiki.js, the editor autocomplete and .book/.index entries already write them docs-root, so no wikilink migration. `[[./a]]` / `[[../a]]` stay doc-relative (already implemented). the rule for the help page:
    - `[x](a.md)`, `[x](./a.md)`, `[x](../a.md)` -> the doc's folder
    - `[x](/a.md)`, `[x](/files/a.md)` -> the docs root
    - `[[a]]` -> the docs root
    - `[[./a]]`, `[[../a]]` -> the doc's folder
  - only hand-typed bare markdown links change meaning - the editor inserts markdown links as `/files/<path>`, so the migration action's list stays short
  - admin action "scan links for relative migration" (like Repair Broken Links): scan every doc for bare links whose target changes with the new rule, list source file / link / old target / new target, let the user select and apply - the apply rewrites them to the docs-root form (`/a.md`) so they keep their old target. business logic in files, thin job/handler wrappers, writeResponse / writeAPIError, translations
  - update ResolveRelativeLink / RelativeLink, rename (renameLinkFunc keeps the written style), relinkMovedDoc (a move now changes bare links too), media relocate (drop the "docs root first, then doc folder" fallback for bare markdown links), book resolveRelativeLinks, help page docs
  - upgrade note: bare links now read from the doc's folder, run the new admin action to keep the old targets
  - links suite cases for the new rule and the migration action
- 4. `../` above the docs root (decision 3: both) - done: pathutils.LinkClimbsAboveRoot, FindBrokenLinks reads the docs' content for them (BrokenLink.AboveRoot), no metadata field
  - keep resolving it clamped to the docs root (renders like a url), and also list it in the broken links scan as "climbs above the docs root" with the clamped path as suggestion, so "Repair Broken Links" can rewrite it
  - ResolveRelativeLink has to report the clamping (e.g. a second return value); FindBrokenLinks only reads metadata, so either store the info in link metadata or scan content for it - decide
- 5. warning noise in folder moves - done
  - `could not get metadata for linked file docs/` (a folder link, now valid in UsedLinks) and `... <file moved along>` (metadata not moved yet, MoveFolder resyncs it afterwards) in files/metadata_links.go (step 3 of updateLinksForMovedFile, MetaDataMutate on movedMetadata.UsedLinks) - skip folder targets (trailing "/") and moved-along files there
- 6. leftovers not on the codec yet
  - table cells bypass the link renderer: server/api_tables.go:132,138 renders headers and cells with parser.RenderInlineMarkdown (plain goldmark), not through RenderLinks - in the interactive table view a `[[note]]` stays literal text, `[x](a b.md)` / `./` links / anchors aren't routed to /files/ and a special-char link breaks. run cells through RenderLinks with the doc's path first (or give RenderInlineMarkdown a docPath), and add a table case to the links suite
  - thememanager/template_data.go "urlPathSegment": hand-rolled url encoder (misses "%" and more), used in themes/builtin/history.gohtml:10 for /files/history/ - use pathutils.ToFileHistoryURL via a template func and delete urlPathSegment
  - dokuwikiconverter/converter.go renderElement case "link" markdown branch (fmt.Sprintf("[%s](%s)")) is dead since processLinks returns parser.Link.String() for markdown - remove it
  - server/render/render_media.go RenderMediaListSelect: onclick="insertMediaIntoEditor(this)" is defined nowhere - either delete the select list (+ its mode in handleAPIGetAllMedia) or insert a server-written data-link like the autocomplete
- 7. guard against divergence - done: FuzzLinkCodec / FuzzRewriteLinksIdentity (parser_fuzz_test.go), TestScannerMatchesGoldmark (parser_goldmark_diff_test.go), TestNoHandRolledLinkHandling (parser_guard_test.go). left: lower the guardRules allow-list once 6 removes api_tables, urlPathSegment and the dokuwiki markdown branch
  - fuzz test (go native fuzzing, seeded with the specialchars corpus) for the codec: for any path and link kind, ParseLink(Link{Path: p}.Dest()).Path == p, and RewriteLinks with an identity callback never changes content
  - differential test scanner vs goldmark: for the corpus and a set of tricky markdown (nested brackets, link text over lines, code spans, ref defs, `<...>` destinations, a destination on the next line), the links ExtractLinks finds must match the links goldmark's AST finds (ast.Link / ast.Image destinations, decoded) - catches the scanner reading markdown differently from the renderer
  - a go test that greps the source (internal/, static/, themes/) and fails on: url.PathUnescape / url.PathEscape outside parser/link_rewrite.go and pathutils, regexes with `\]\(` or `\[\[` outside internal/parser (allow-list dokuwiki syntax), hand-built markdown links (`"](" +`, `"[%s](%s)"`), ReplaceAll with "%20", encodeURIComponent on link paths in js, goldmark.New / .Convert outside an allow-list of render paths known to run RenderLinks first (that's how the table cells slipped through) - with an allow-list, so a new hand-rolled reader/writer fails the build instead of relying on review
- 9. found by the tests of 1 and 7 (not fixed yet, decide)
  - codec: ascii control characters besides tab / line breaks aren't percent-encoded in link paths - a "\f" ends a markdown destination, so the link is lost (fuzzPath skips them)
  - codec + walker: "`" isn't encoded - two of them in a path are a code span for maskCode, so the walker skips the link, goldmark reads them as part of the destination (fuzzPath skips them, a knownDivergences case)
  - walker vs goldmark (knownDivergences in TestScannerMatchesGoldmark): a destination, title or reference definition destination on the next line (goldmark: link, walker: none), an escaped "\[" before "](dest)", a link in an indented code block or an html comment (walker: link, goldmark: none)
  - markdown images still render through renderImage / resolveMediaPath, not LinkTarget: a bare `![x](pic.png)` that exists only in the docs folder renders /media/pic.png (404) while link metadata reads docs/pic.png, and an image named with a trailing space ("trail.png ") renders nothing
  - other hand-rolled link readers / writers (allowed in guardRules): pdfexport/images.go zoneImageLinkRe (header/footer zone image template), dokuwikiconverter/converter_process.go builds an unencoded `[url](url)` /browse/folders link, server/render/render_editor_codemirror.go strips link syntax to text with its own regexes
- 8. run every headless suite after the changes (`knov --start-tests --remove`), not only links - two go tests in internal/server/render (TestHeaderContextMenuScript_LabelsMapToTheirOwnAction, TestRowContextMenuScript_LabelsMapToTheirOwnAction) already fail before the link refactor (table editor), unrelated

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
