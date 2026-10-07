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
  - admin action scan for broken parents/grandparents (e.g. i have grandparent for a file that no longer exists? how is this even possible?) and why is the file not shown as a child in the parent metadata? and why do i have an ancestor in the kanban board that no longer exists and doesnt have any children? => maybe a new admin action cleanup_metadata?
  - general and centralized solution for links (link rewrite, parser..) with all of the special chars, we have multiple different solutions for different special chars
  - make the upgrade.md file more readable
- test
  - remote git in mobile

# reserved folders refactoring

- todo: make docs paths unambiguous so no top-level docs folder name has to be reserved
  - problem: pathutils.parsePath guesses the type from free text - a leading "files/" is stripped, "media/..." is read as a media file and "docs/..." as a docs file. a docs file at data/docs/media/x.md (or docs/docs/..., docs/files/...) therefore can't be resolved back to itself
    - current workaround (keep until this is done): configmanager.ReservedDocsFolders + pathutils.CheckNewDocsPath/ErrReservedPath reject creating/moving files there (handlers, files.MoveFileNoRefresh/MoveFolder, configeditor.CleanID, validateKanbanFolder), existing files are exempt via os.Stat
    - the workaround only covers files created by the app - files that arrive via git pull/sync or are copied into the filesystem are still broken:
      - contentStorage.ListFiles + files.pathsToFiles list them as "media/x.md" without a docs/ prefix
      - File.ViewURL points to /files/x.md => docs/x.md (404 or the wrong file)
      - their metadata key "media/x.md" is the same key as a real media file data/media/x.md, so the two overwrite each other's metadata
  - fix:
    - docs paths always carry an explicit "docs/" prefix internally (listing, metadata keys, File.Path)
    - user input (form paths, rename/move targets) and /files/<rel> URLs are treated as literal docs-relative paths - no prefix stripping (e.g. a separate docs-rel => full path function next to ToDocsPath)
    - keep the prefix guessing only where it's really needed (links in content / old metadata)
    - catch: a link like [[media/x.md]] still resolves to media, so docs files in docs/media/ need a "docs/media/x.md" link - decide and document this
  - migration: metadata keys of docs files that currently collide with media keys, and filter/tracker configStorage ids starting with docs/, media/ or files/ (see configeditor.CleanID)
  - afterwards delete: ReservedDocsFolders, CheckNewDocsPath, ErrReservedPath, writeReservedPathError/reservedPathMessage, the reserved check in configeditor.CleanID and validateKanbanFolder (+ the hint in the Auto-Create Tags setting Desc), and turn TestCheckNewDocsPath into "these paths now resolve correctly" tests
  - touches many ToDocsPath/ToWithPrefix/ToRelative callers - do it as its own refactor, run `--start-tests --remove` afterwards

# link refactor

- goal: one general, centralized solution for links and their special chars - one reader, one writer, decoded paths everywhere in between
- done (step 0): parser.DecodeLinkPath / EncodeLinkPath as the only link codec, utils.NormalizeLinkPath + WithDefaultLinkExt, RewriteLinks skips external links, rename/media relocate/filter index/book editor write encoded paths (see the CLAUDE.md rule)
- special chars are 6 separate contexts - only 1-4 belong to this refactor:
  1. link text in content (`](dest)`, `[[...]]`, html src/href) - parser codec
  2. links written by js - wiki-autocomplete.js inserts a wikilink path raw (not encoded at all) and a markdown/media target with encodeURIComponent per segment but "#" kept as separator, codemirror upload inserts `](media/<path>)` raw
  3. app urls - pathutils.To*URL, but raw url.PathEscape(relPath) is left in render_files.go / render_media.go ("/" becomes %2F), js builds urls in several ways (panel-file.js, panel-tree.js)
  4. anchors - percent-decoded in 4 places (headings.go anchorText, ProcessMarkdownLinks twice, pdfexport/renderer.go resolveLink)
  5. allowed filename chars - only SanitizeFilename on uploads, no shared policy for create/rename/import
  6. output escaping (html, js, csv, md table cells) - already central enough, leave out
- [x] step 1: shared special-char corpus as the test fixture
  - corpus: `internal/test/specialchars` (Names, ValidOn) - the todo list plus `a<b>.md` and a special-char folder `x (1)/ö ü.md`
  - go tests: parser TestSpecialCharLinksRoundTrip (encode/app url -> ExtractLinks, ResolveWikiTarget, Parse+Render href/media preview), book TestFileRefRoundTrip (EncodeFileRef -> ToMarkdown -> Parse -> ResolveWikiTarget/DecodeFileRef/ComposeEntries), files TestRenamedLinkReadsBack (renameLinkFunc to and from each name), dokuwikiconverter TestSpecialCharLinks
  - in-app suite `links` (internal/test/linkstest): autocomplete (real wiki-autocomplete.js in chromedp), upload, rename + rename back, media relocate, filter index, book editor api - each link form in its own doc, checked through used links, linked from and the rendered page
  - gaps found (these decide steps 2-5, the failing tests are the acceptance list):
    - leading/trailing space - every reader trims after decoding (ExtractLinks, ResolveWikiTarget, render), so `%20lead.md` / `[[trail.md ]]` read as `lead.md` / `trail.md` - in every writer (codec, rename, relocate, filter index, book editor, autocomplete)
    - `&` - only `[x](/media/a&copy;b.png)` (EncodeLinkPath of a /media/ path): ProcessMarkdownLinks passes /media/ links to goldmark untouched, which resolves the entity -> a©b.png. docs links, images and pathutils urls are fine (PathEscape encodes the ";") - the step 0 finding below was only half right
    - book editor - the plain `path#anchor` value is split at the first `#` / `|`: `a#b.md` resolves to `a.md` (not included), `a|b.md` resolves to `a.md` as alias and includes that other file
    - rename (not char related, every name) - MoveFileNoRefresh passes the docs-relative oldPath, renameLinkFunc compares it to NormalizeLinkPath(p), which is `docs/`-prefixed for a `/files/` url or html href - so those links are never rewritten on a rename through the api (files_test used `docs/` paths on both sides and missed it)
    - media relocate (every name) - a docs-root markdown link without leading "/" (`![x](sub/a.png)`, the form EncodeLinkPath / rename / filter index write) isn't relocated: relocate reads a bare markdown path as doc-relative only, metadata and render read it as docs-root. doc-relative, wiki and html /files/ links are relocated
    - upload (every special-char folder, even a space) - context_path comes from location.pathname, percent-encoded, and UploadMedia doesn't decode it -> media lands in `media/.../x%20%281%29/pic.png`, and the raw inserted `media/<path>` link decodes back to the (missing) mirror folder
    - autocomplete wiki - raw path: `a%41` -> aA.md, `a#b` -> a.md + anchor, `a|b` -> alias, `[1]` ends the wikilink (reads the folder), `a\b` -> a/b.md
    - autocomplete markdown / media - "#" isn't encoded -> `a.md` + anchor / `media/.../a`
    - filter index - the label is the raw path, `[1].md` makes `- [.../[1].md](...)` no link in the rendered page (metadata is fine)
    - dokuwiki - `{{:[1].png}}` -> `![[1].png](...)`: the alt text starts a wikilink, ExtractLinks reports an extra link "1"; `[[v1.2 notes]]` -> /media/v1.2%20notes (own extension rule, not WithDefaultLinkExt)
    - reader mismatch - `/files/x` without extension: metadata adds .md, render links `/files/x` (ProcessMarkdownLinks' /files/ branch skips WithDefaultLinkExt)
    - no gap: `%`, `%41`, `?`, `( )`, quotes, `< >`, `:`, `\`, unicode, `v1.2 notes.md`, the nested folder - through the codec, rename (markdown/wiki), relocate (doc-relative/wiki/html), filter index metadata, book editor
- [x] step 2: one link model + one scanner/formatter in parser
  - parser.Link (Kind, Image, Text, Path decoded, Query, Anchor, Alias, Title kept as written, External), ParseLink reads one destination / wikilink body, Link.Dest writes it back, Link.String writes a new link (escapes `[ ]` in the text) - decodeLinkPath/encodeLinkPath are their internals
  - RewriteLinks callbacks get a Link and return the new decoded path, RewriteLinks encodes it and only rewrites when the decoded path changed; a rewritten `<...>` destination keeps its angle brackets (path encoded)
  - removed: DecodeLinkPath, EncodeLinkPath, SplitWikiTarget, splitLinkPath, splitMarkdownLinkDest; one wikiLinkRe (needs the closing "]]") for scanning and rendering
  - codec: raw text trimmed, then decoded - nothing trims a decoded path anymore (ExtractLinks, NormalizeLinkPath, parents trim the form value instead); wiki edge spaces encoded; markdown/html resolve complete html entities like goldmark, "&" written as %26; markdown no longer encodes quotes
  - left out on purpose: no position field (nothing needs it), anchors stay as written (normalized once in step 3 with their readers), the bare markdown path rule is decided in step 3 with relocate
  - fixed gaps: leading/trailing spaces (codec, rename markdown/wiki, relocate wiki, filter index), `&` in /media/ links, filter index `[1].md` label, dokuwiki `{{:[1].png}}` alt text (no "]]", no wikilink), `a%41` re-normalized by RewriteLinks
  - known: the links suite's RefreshCaches rebuilds still run when `--remove` deletes the isolated storage, so "unable to open database" errors are logged after the summary - harness noise, not a result
- [x] step 3: move every reader onto the scanner
  - metadata (ExtractLinks), rename and media relocate were already on RewriteLinks since step 2; RewriteLinks, ResolveWikiLinks and ProcessMarkdownLinks share one code-skipping walker (replaceOutsideCode), ProcessMarkdownLinks reads each link with ParseLink and writes its app url (/files/ with WithDefaultLinkExt also for `/files/x`, /media/ through ToMediaURL)
  - anchors are decoded once (Link.AnchorText) and turned into a heading id by AnchorID(text) - render (also pure same-page anchors), book, pdfexport resolveLink; pdfexport reads image / zone image destinations with ParseLink
  - book entries: DecodeFileRef / EncodeFileRef take path and section apart, the editor has a section field (`entries[][section]`) - no plain `path#anchor` split anymore
  - rename compares metadata paths (ToWithPrefix both sides); relocate reads a bare markdown path docs-root first like it renders, doc-relative only as fallback (html relative links stay doc-relative)
  - dokuwiki: links and media written with Link.String (Link.Image), a `[[...]]` link is a page unless its extension is a known media type
  - step 2 leftovers: ParseLink only takes the angle branch with a closing ">", and a bare markdown destination never starts with "<" in the scanner (CommonMark) - `[x](<abc.md)` is no link for rendering, metadata and rename; Link.Image used (dokuwiki, linkstest)
  - fixed gaps: book `#` / `|` names, dokuwiki `v1.2 notes`, links suite rename (file url / html), relocate (markdown docs-root), book editor, `/files/x` without extension on render, links / wikilinks in code rendered as links, a percent-encoded image path in the pdf export
  - left out on purpose: dokuwiki doesn't use WithDefaultLinkExt - a dokuwiki id is always a page, WithDefaultLinkExt reads `v1.2 notes` as having an extension (that rule stays, see upgrade.md); ResolveWikiLinks still writes an intermediate `[text](/files/...)` that ProcessMarkdownLinks reads again; a book section is stored as typed (a `|` or `]]` in it breaks the entry) and a hand-written `|alias` on an entry is dropped by the editor
  - review fixes: ProcessMarkdownLinks matches on the whole content with code masked (maskCode), so an image alt / link text spanning lines or holding `code` keeps its "![", a linked image `[![b](i.png)](a.md)` or one after a stray "[" stays an image; `[x]( <a.md)` is no link either; dokuwiki media check uses knov's fixed media table (types.MediaCategory), not the host mime db
  - known: a reference definition whose title holds a code span isn't rewritten; reference definitions `[id]: dest` are still rendered by goldmark as written (metadata reads them docs-root); raw html is never rendered (goldmark without WithUnsafe), so the links suite checks html forms through metadata only; the book editor case skips names with a trailing space (the typed path is trimmed, step 5); links suite still failing: autocomplete, upload (step 4)
- [x] step 4: js and urls
  - autocomplete apis (/api/files/autocomplete, /api/files/headers, /api/media/autocomplete) take `link=wiki|markdown` and return each suggestion's ready-to-insert link text (AutocompleteItem.Link, `data-link`) - files and headings through parser.FileLinkDest (wikilink body or markdown /files/ url, empty path = same-page anchor), media through Link.Dest; `data-value` stays the plain path for path inputs. wiki-autocomplete.js inserts `data-link` as is - encodePathSegments, buildTarget, buildMediaTarget and the unused global buildTarget are gone, cursorOffset reads the link text (a "#" in a name is encoded, so only a real anchor keeps the cursor after the link)
  - upload: uploadMediaBlob sends location.pathname as context_path, the handler reads the doc with pathutils.FileFromURL (decoded once, /files/new/... -> 400), UploadMedia gets the docs-relative path and returns MediaUploadResult.Link (parser.Link.String, image link for image/*) - the editor inserts it as is; the js "save first" pre-check is gone (the api answers it)
  - urls: pathutils.ToRouteURL(route, rel) for path routes - replaces the raw url.PathEscape in render_files.go (delete file / folder) and render_media.go (delete media); RenderFileDropdown uses file.ViewURL() instead of `'/files/'+path` in js. builtin theme js: one pathURL(route, path) (per-segment encodeURIComponent, the js side of ToRouteURL) for every path route in panel-file.js / panel-tree.js, $store.filePanel.editURL replaces editPath; query params stay encodeURIComponent
  - fixed gaps: links suite autocomplete (wiki raw path, "#" in markdown/media) and upload (every special-char folder), rename / move / delete / rebuild from the file panel for names with "#", "?" (the path was sent raw), metadata rebuild route for names whose encoding differs from go's (it read chi's still-encoded wildcard, now r.URL.Path like the other path routes), a stale dropdown list inserting the other syntax's link text (hide() now drops the items)
  - left out on purpose: dispatchFetch still decodeURIComponent()s the typed link text to build the search query (reading, not writing a link - a typed `%23` splits as anchor); RenderMediaUploadComponent is dead code and still documents the old context_path; the example theme has no js that builds path urls
  - known: the metadata suite's sanitize-kanban-tags fails on main too (unrelated); the links suite's "unable to open database" lines after the summary are the step 2 harness noise
- [x] step 5: filename policy
  - pathutils.CheckNewNames / ErrInvalidName: a file or folder name of a host path that doesn't exist yet must not hold `# ? | [ ] \` or start/end with a space - existing parts of the path (git sync, manual copy) aren't checked, so a new file in an existing `a#b/` folder is fine. CheckNewDocsPath runs it after the reserved folder check, so every create/move that already called it is covered: file save (create), rename / set path, move folder (job, kanban status rename), chat move, list / book / index editor, filter and tracker (configeditor Kind.Set checks the paired file before storing the config); MoveMediaFileNoRefresh checks the media path (media rename / set path)
  - a move that keeps the name (to another folder: kanban card drag, move folder, set path) only checks the new folder, files.keepsInvalidName - an existing bad name can still move
  - server: writeReservedPathError -> writeNewPathError (400 for both errors, translated message from newPathMessage), handleMoveError answers 400 for both
  - links suite rename: a name CheckNewDocsPath rejects is written through storage (like git sync) and only renamed away, that MoveFileNoRefresh rejects it is covered by files.TestKeepsInvalidName (pure, temp dir); forms written with parser.Link instead of raw text
  - step 4 leftovers: media rename / rename-form / path-display read r.URL.Path instead of chi's still-encoded wildcard, pathURL moved from panel-file.js to rail-core.js (shared by panel-file.js and panel-tree.js)
  - left out on purpose: no import mapping - the dokuwiki converter is an export (zip) and dokuwiki page ids can't hold these chars, backup restore brings back existing files like git sync; upload keeps utils.SanitizeFilename (already only keeps `a-z0-9-_.`), its media folder mirrors the doc's existing folder and isn't checked, same for media relocate; convert-to-markdown keeps the existing name; kanban statuses are already `[a-zA-Z0-9_]`; the codec is unchanged
  - book editor skip stays: the typed path is trimmed, which is right now that the app never creates a name with an edge space - only a git-synced `trail.md ` can't be a book entry
  - known: the kanban suite fails 6 cases on main too with the copied settings (statuses like `innboxx`) plus the known sanitize-kanban-tags; the links suite's "unable to open database" lines are the step 2 harness noise
- step 5b: leftover from the step 5 review
  - fixed: CheckNewNames stops at the docs / media root (a missing root or a data path with `#` no longer fails every create), configeditor Kind.Set checks CleanID's id directly, pathutils TestCheckNewNames pins every corpus name (the links suite's rename skip reads the policy)
  - known: kanban moveFileUnique's collision fallback for an existing bad card name (`a#b.md` -> `a#b_2.md`) is a new name, so the card move fails with ErrInvalidName - rare (git-synced name + same name already in the status folder), fixing it needs a "keeps the old name" heuristic
  - handleMedia (pages_browse.go, `/media/*`) still reads chi.URLParam(r, "*") - the still-encoded wildcard when the request has a RawPath, the bug fixed for the media rename / rename-form / path-display and metadata rebuild routes; switch it to r.URL.Path like them and check a media name whose encoding differs from go's (links or media suite)
- step 6: automatic metadata rebuild on upgrade
  - an option for me to request a one-time full metadata rebuild for certain versions/cases (e.g. a link-reading version constant bumped whenever the way links are read changes), persisted so startup compares it and runs metadata-full-rebuild once when it differs
  - then the upgrade notes for link reading changes (step 0, 2, ...) can say "nothing to do" instead of asking for a manual rebuild
- step 7: repair media from the old upload (needs the rebuilt link metadata of step 6)
  - the old upload stored media under the still-encoded folder (`media/x%20%281%29/pic.png`) and inserted the link raw, which now reads as the missing `media/x (1)/pic.png`
  - broken links: FindBrokenLinks also suggests the file at the link path read literally (not decoded) when it exists - an exact match, unlike the unique-basename guess that misses common names like `image.png`; the existing repair rewrites the link (the encoded folder stays)
  - optional: the misplaced media scan also reports media whose folder doesn't mirror the one doc linking it, moved and relinked through the same review-then-relocate flow - fixes the folder itself and covers files copied in by hand; never automatic, a folder literally named `x%20(1)` can be intended
  - upgrade note in docs/upgrade.md: point at the broken-links repair instead of moving the media by hand
- open findings from the step 0 review to fix along the way
  - stale link metadata after upgrading needs a manual metadata-full-rebuild - see step 6
- run `go test ./...` and `--start-tests --remove` after every step

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
