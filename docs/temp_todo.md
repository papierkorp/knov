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
  - add a /system/environment (/system/environment_info) page which includes ALL environment vars (we currently have it in /admin visible => maybe we can use the new system page there as well so we dont loose it there?) with a description, also i would like to add a script/tool (in the tools folder) which automatically creates the .env.example file with the description, i dont know what the best format is maybe a table? => eventually we will need to refactor the env system a little to create all of this
- fixes
  - media rename?
  - sidebar is not working in /chat
- chore
  - async jobs follow up candidates
  - switch to static id instead of using the path
  - show if auto backup is enabled in the /system/backup page and the backup slideout

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

Also give your opinion about the changes

# x

Review: HidePaths scope-tagging feature + settings validation error propagation

**Summary of the change:** adds per-scope (`::tree|browse|overview|search|filter|kanban`) opt-in exceptions to `HidePaths`, threads a `scope` parameter through `FilterByVisibility`/`IsPathHidden` to every call site, and — as a apparently-supporting change — makes `setFromJSON` return an `error` instead of logging-and-swallowing validation failures, which now propagates out of `InitSettings` and `ImportSettingsJSON`.

### Problems

**1. `ImportSettingsJSON` is no longer atomic and now partially mutates live state on failure (`internal/configmanager/settings.go:118-131`).**
The loop iterates `allSettings` and calls `s.setFromJSON(val)` for each key present in the imported JSON, applying the value directly to the setting's live `atomic.Value` as it goes. Previously, a per-setting validation failure was logged and skipped, so the loop always ran to completion and `SaveSettings()` was always reached — the import was "best effort." Now, the first setting that fails validation causes an immediate `return err`, which (a) leaves every setting processed *before* the failing one already mutated in memory with the new imported value, since `s.val.Store` already ran, and (b) never calls `SaveSettings()`, so none of it is persisted. The result is a live process running on a hybrid of old and newly-imported settings that matches neither the old persisted file nor the imported file, until either a full restart or another explicit save. This is a real regression in behavior, not just a stricter validation — the previous version guaranteed the loop always completed; this one can stop partway through with observable side effects and no rollback.

**2. Fail-fast settings loading now applies to every validated setting, not just the new `HidePaths` scopes, with no in-app recovery path (`internal/configmanager/settings.go:14-30`, `main.go:147-150`).**
`IntSetting`/`StringSetting`/`StringSliceSetting.setFromJSON` all now return an error on validation failure instead of logging and keeping the previous/default value. Since `InitSettings` returns on the first such error and `main.go` treats that as fatal (log + `return`, i.e., refuse to start), a single persisted value that fails its bound check (e.g. `PageSize` outside 5–200, `MaxUploadSizeMB` outside 1–100, any hex-color setting) now prevents the whole application from starting, whereas before it silently fell back to that one setting's default and the app came up normally. This mirrors the existing fail-fast convention used for the storage `Init` calls earlier in `main.go`, so it's architecturally consistent with that pattern — but it's a meaningful behavior change specifically for settings, and for a self-hosted single-user app the only recovery is manually editing/deleting the persisted settings file, since there's no in-app way to reset a single bad key once the process won't boot. Worth confirming this fail-fast tradeoff is intentional for settings specifically (the error message does include the offending key/value, which helps, but there's no server running to expose it anywhere but the log).

**3. `handleAPISetSetting` passes a fully-rendered, dynamic error string into a translation "key" (`internal/server/api_settings.go:148`).**
`translation.SprintfForRequest(lang, key, args...)` (`internal/translation/translation.go:52-68`) is a thin wrapper over `x/text/message.Printer.Sprintf`, which treats its second argument as a message-catalog key and, on a catalog miss, falls back to using it as a `fmt`-style format template against the supplied `args`. Every other call site in the codebase (`api_chat.go`, `api_cronjob.go`, etc.) passes a static literal string with dynamic values as separate `args`. This new call instead passes `err.Error()` directly with zero args — and that error string is built by `fmt.Errorf` in `ValidateHidePaths`/`IntSetting.validate`/`StringSetting.validate`, embedding the actual user-submitted form value (e.g. the invalid hide-path entry text) into the string. Two consequences: it can never match a real catalog key (so "translation" is a no-op here, just extra indirection), and if the user-submitted value itself happens to contain a `%` followed by a verb-like character (e.g. a hide-path pattern like `100%done::tag`), `Printer.Sprintf` will try to consume it as a format verb with no corresponding argument, producing a garbled error message (Go's `fmt`-style formatting inserts `%!s(MISSING)`-style placeholders rather than crashing, but the user-facing text becomes wrong). This is a plausible, concrete correctness issue introduced specifically by this line, not a translation-content nitpick.

### What looks solid
- The core scope logic (`splitHidePathEntry`, the `len(tags) > 0 && !slices.Contains(tags, scope)` guard in `IsPathHidden`, and validation via `ValidateHidePaths`) is correct and consistent: I traced every `FilterByVisibility`/`IsPathHidden` call site and confirmed each was updated to pass the right scope constant (or `""` for `media` and the metadata aggregate functions, matching the documented intent), and the project builds cleanly.
- Test coverage for the new feature is notably thorough — `browsetest`'s new `scope_endpoints.go` spins up the real router over `httptest` and hits `/api/files/tree`, `/api/files/list`, and `/api/files/folder` to prove each handler is wired to its *own* hardcoded scope constant rather than trusting a shared helper, which is exactly the kind of regression a future edit could silently introduce. `filtertest`/`kanbantest`/`searchtest`/`metadatatest` each add a matching scoped-hide case for their own call site.
- No injection, secret-exposure, or O(n²) concerns in the new matching logic; the added `slices.Contains` checks are bounded by the tiny fixed `hideScopes` list.
- Architecturally, the scope feature stays in `configmanager` (owning package) with server/job code doing plain pass-through of a constant — consistent with the project's thin-handler convention.

I'm not issuing "VERDICT: APPROVED" — items 1 and 2 are real behavior regressions worth a second look before merging, and item 3 is a concrete, if narrow, correctness bug.

what would be a good solution for those 2 problems? do we revert the change  that stops the app from starting and make it only work for the visibility?

Good instinct, but the fix isn't "make it fatal only for HidePaths" — it's "don't make load-time validation fatal at all, for any setting."

Here's why: the actual save-time gate that matters for the typo-rejection feature (`::serach`) is `SetFromString`, which is what the settings API already calls via `handleAPISetSetting`. That path already returns an error to the caller and rejects the bad value before it's ever persisted — it works today and doesn't need touching. The fatal behavior you want to revert is a *different* code path: `setFromJSON`, used only when *loading* already-persisted/already-exported data in `InitSettings` and `ImportSettingsJSON`. Data reaching that path was already validated once when it was saved, so treating a load-time validation miss as fatal buys little (it mostly guards against hand-edited config files) at the cost of bricking startup on any stale/out-of-range value.

So the fix for both problems is the same small change, made in one place:
- In `InitSettings`, go back to logging a warning and continuing to the next setting when `s.setFromJSON` fails, instead of returning the error up to `main.go` as fatal.
- In `ImportSettingsJSON`, do the same — skip the bad key and continue the loop instead of aborting, then still call `SaveSettings()` at the end. That also fixes problem 1 for free: since nothing aborts mid-loop anymore, there's no partial-mutation-without-persist state — every setting gets attempted and the good ones get saved, exactly like the pre-change behavior.
- Leave `setFromJSON`'s `error` return type as-is (still useful for logging which key/value failed) and leave `SetFromString`/`ValidateHidePaths` untouched — that's the part that's actually working correctly.

Want me to make that change?
