# Project

- im currently working on the following golang, htmx app.

## Safety

- DO NOT EVER MAKE ANY CHANGES IN "/media/markus/SamsungT5/knov/data"
- dont touch or use the prod environment which is using the Port 1325

## Working style

- i want you to anwser with as little code as possible to only fix the problem i anwsered without any unecessary code, as simple and small as possible with as few changes as possible
- check current status of the project
- always search the project since you already have all the files
- no need for backwards compability since the app is not released yet - you can remove functions/routes
- no need to update the changelogs since they are auto generated per git commits
- for breaking changes, deprecations or removals also add a user-facing upgrade note to docs/upgrade.md (what changed, what the user has to do) - `make release` moves it into docs/releases/<version>.md as "upgrading from vPrev to vX.Y.Z", resets docs/upgrade.md to its template, and fails if there are breaking commits but no notes. the /system/release version range picker shows every release after the user's version up to the target, so each release's upgrade notes only need to cover that one step
- there is background automation on my machine that watches git commits and auto-commits changes to docs/changelogs/, docs/releases/unreleased.md and docs/temp_todo.md (and can auto-commit other pending working-tree changes along with them) - if you see commits you didn't make, or changes to those files you didn't write, that's this automation, not a bug

## Architecture

- business logic belongs in the package it's about (files, git, filter, etc.) - server (handlers) and job should stay thin wrappers: validate/resolve input, call one function in the owning package, translate the result into a response or JobRun
- same for the job, the job package should just include a wrapper
- i dont want any html generation in the handler - use the render subpackage for any html strings
- if anything related to paths prop up - use the pathutils package!
- never hand-build /files/, /files/edit/, /media/, /files/edittable/, or /files/history/ URLs with string concat or fmt.Sprintf - use pathutils.ToFileURL / ToFileEditURL / ToMediaURL / ToFileEditTableURL / ToFileHistoryURL instead
- if working with paths - we have to take care of both linux and windows os paths
  - real filesystem paths of the host (os.ReadDir, filepath.Walk/Join, DataPath, ...): use filepath / pathutils (pathutils.ToSlash, PathContains, ...) - they follow the host OS rules, on linux "\" is a valid filename char and must not be converted
  - path text written by users or other systems (links in content, imported wikis, settings values, form input): use pathutils/crosspath (crosspath.ToSlash, crosspath.IsWindowsAbs) - converts/detects windows and linux syntax the same on every host OS. crosspath has no knov imports, so it can also be used where pathutils can't (e.g. configmanager)
  - never hand-roll ReplaceAll(`\`, "/") or drive-letter checks, use crosspath
- link paths inside content (markdown `](dest)`, `[[wiki]]`, html src/href) are decoded exactly once when read and encoded exactly once when written - read them with parser.ParseLink / RewriteLinks / ExtractLinks (they hand you a parser.Link with the decoded Path), write them with parser.Link (Dest for a destination, String for a whole new link) or by returning the new decoded path from a RewriteLinks callback. everything in between (metadata, RewriteLinks callbacks) holds decoded paths, so never url.PathUnescape / percent-encode / unescape markdown a link path again or hand-roll an escaper for it - a decoded link path like "/files/" + path is fine there, the pathutils url rule is for encoded urls. map a decoded link path to a metadata path with utils.NormalizeLinkPath
- we updated to htmx 4.0 so the syntax is different to htmx 2.0 be careful of this
- env vars (configmanager/envdefs.go, KNOV_* prefix) are for deployment/admin-level config only - things set once before startup and needing a restart to change (ports, paths, storage providers, credentials, sync intervals). runtime-editable, user-facing preferences (toggles, exclusion lists, display options shown on the /settings page) belong in configmanager's settings registry (settings_registry.go, StringSetting/BoolSetting/IntSetting/StringSliceSetting) instead - persisted via configStorage and editable without a restart. when in doubt: if it should show up on the settings page, it's a setting, not an env var

## API

- make the api RESTful
- always return an honest HTTP status code: real 4xx/5xx on failure (use writeAPIError), never a soft 200 with an error body. htmx 4 swaps 4xx/5xx response bodies by default, so the inline error still shows. a valid request that legitimately has no data is a 200 empty-state, not an error
- if you create an api call keep in mind to keep it theme friendly (lean more towards being generic) and also add comments for swagger to work, also stay with accept form data we dont need to accept json
- for every return in the api folder use: writeResponse (success) or writeAPIError (failure)
- dont forget to use translation.SprintfForRequest in the server package for handler and the render package for EVERY String

## Logging

- for logging message i only want use lowercase
- use the logging package with a appropriate key

## Frontend / theming

- think theme-agnostic
- for styles/css files use Global styles only in style.css and all specific files use ID selectors (#page-, #component-, #view-)
- dont use hardcoded colors use the available vars instead
- goal: less hand-rolled JS, more htmx
- if you add a new env also add it to .env.example and the html templates of both themes
- keep the usage of alpine js in mind if useful
- DONT USE HARDCODE COLORS, ALWAYS use the color vars which are provided per defaults.css file

## Go style

- Use fmt.Fprintf(…) instead of WriteString(fmt.Sprintf(…)) (QF1012 default)
- if possible use slices.Contains instead of loops

## Testing

- see docs/testing.md - pure logic / anything a `TestMain` can point at an `os.MkdirTemp` dir goes in a normal colocated `_test.go` file (`go test ./...`); anything needing real installed files or a live app instance goes in an `internal/test/<x>test` suite run via `knov --start-tests`
