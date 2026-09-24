# temp todo

# small stuff

- features
  - create a system for themes (another repoistory with themes)
    - e.g. https://github.com/papierkorp/knov_themes
    - e.g. create a table/dict with all top level folders - than check if there is a theme.json
  - implement a 2 view system (e.g. todo list in raw markdown/vs rendered todolist, or the new tracker editor => clicker vs statistics)
  - multiview in theme
  - a collection/library for books so i can download multiple books with one click
  - toc in codemirror edit all
- fixes
  - html-escape user-controlled values in render_kanban.go (tag names in chip title/body/data-tag, card titles, file paths in data attributes) - currently interpolated raw via fmt.Fprintf, so a tag/title from frontmatter can inject html
- chore
  - kb-status-inbox is not shown in info slideout as a tag
  - unify tag counts on one way: the dashboard tag widget computes live (GetAllTagsExcludingHiddenFiles), while /api/metadata/tags reads the tag-count cache (CacheKeyTagCounts, unscoped only). simplest: drop the cache read in the tags handler and compute live (cheap now - one pass over the cached file list). not done yet since other code still reads that cache, incl. the dashboardtest and metadatatest suites
- test
  - remote git in mobile



# every other time

- take a look at all routes if we use writeResponse everywhere neccessary and if we can update the functions where we only use json to htmx as well if its useful
- take a look at the whole codebase into all javascript snippets/scripts with the goal of reducing javascript in favor of more htmx - im also fine with refactoring to make this to work since i think we already use a lot of javascript which could be resolved using htmx
- pass over css files (components.css/panels.css/layout.css) for dead selectors, confirm remaining ones follow the id-selector convention
- check the whole codebase for hardcoded colors and replace theme with the vars provided by the defaults.css file
- add all missing german translations

# mobile wrapper

description: replace the termux-based mobile setup with a proper installable android app. instead of running the go server through termux, wrap it as a native android shell app so it runs as a real background app with an icon, no terminal needed. considered gomobile bind (compiling go as a jni library) but decided against it - unnecessary complexity since the go binary itself needs no changes. instead: cross-compile the existing go server for android and run it as a plain subprocess from a foreground service, with a webview pointed at localhost. lives in its own folder in repo root (e.g. /android), separate gradle/kotlin toolchain, not integrated into the go build.
use the tools/docker_android for development

- [x] create `/android` folder at repo root with a minimal android studio project (kotlin) - scaffolded, not opened/synced in android studio yet
- [x] add `.gitignore` entries for android build artifacts (build/, .gradle/, local.properties, *.apk, *.aab) and never commit signing keystores
- [x] cross-compile the go server with `GOOS=android GOARCH=arm64 go build` and get the binary into `android/app/src/main/jniLibs/arm64-v8a/libknovserver.so` (`make mobile-apk-binary`) - not `assets/` as originally planned, see next item
- [x] handle android's sandboxed storage - no go code changes needed, existing KNOV_* env vars already cover this; ServerService.kt points them at subfolders of the app's private filesDir
- [x] ~~on first app launch, copy the bundled binary into the app's private files dir and chmod it executable~~ - doesn't work: since android 10, apps can't execute a file they wrote to their own data dir at runtime (W^X), even after chmod +x, confirmed on a real device (`Permission denied`, error=13). fixed by shipping the binary as a "native library" (jniLibs) instead of an asset - the OS extracts those executable at install time. required `packaging { jniLibs { useLegacyPackaging = true } }` in app/build.gradle.kts so it's actually extracted to disk rather than mapped from inside the apk
- [x] implement a foreground service that starts the binary via ProcessBuilder and shows the required persistent notification (ServerService.kt)
- [x] implement a WebViewActivity that loads http://127.0.0.1:<port> once the server is up (MainActivity.kt, polls until reachable)
- [x] restart behavior on crash - service now relaunches the process via a `waitFor()` watcher thread if it dies unexpectedly (ServerService.kt); binary is already re-extracted by the OS on every app (re)install, so no separate version check is needed
- [x] added a minimal `Theme.AppCompat.DayNight.NoActionBar` (res/values/themes.xml) - MainActivity crashed on launch without it ("You need to use a Theme.AppCompat theme"), found on real-device test
- [x] built and run for real - JAVA_HOME needs to point at android studio's bundled JBR (system java 25 is too new for AGP 8.13.2/gradle 9.3.0); `android.builder.sdkDownload=false` in gradle.properties needed too, the installed SDK's newer package.xml format isn't understood by this AGP's bundled sdklib and it tried (and failed) to redownload build-tools instead of using what's already installed
- [x] test on a real device (sideloaded via `adb install`, no play store) - confirmed via logcat + curl through `adb forward`: server starts, binds to 127.0.0.1:8324, webview loads it (HTTP 200). visual on-screen confirmation still pending, device's wireless adb dropped when its screen locked mid-session
- [x] app icon - generated mipmap-*/ic_launcher(_round).png from the existing static/knov_logo.png, wired into the manifest (no separate android asset pipeline needed)
- [x] launch/error feedback - MainActivity shows a plain "starting server..." message in the webview itself while polling (no separate layout needed); after 15s still-not-up shows "still trying..." but keeps polling and auto-loads the real page once it comes up. ServerService now also retries if the very first exec attempt itself fails (previously only retried after a successful start later crashed)
- [x] stop action on the persistent notification (previously it just said "Knov server running" with no way to stop it from there)
- [x] in-app `.env` editor (EnvEditorActivity) - the go binary already applies a `.env` file from its cwd onto the process env unconditionally (loadEnvFile in configmanager/config.go), and the wrapper's cwd is the app's private filesDir, but there was no way to get a file in there without adb. small ".env" button on the main screen opens a plain text editor, saves to filesDir/.env, restarts ServerService
- [ ] check battery-optimization behavior on real OEMs (some kill foreground services aggressively despite the notification) - needs a real device, couldn't verify from this session
- [ ] make the data/storage folder accessible from a normal file manager on the phone - one of the main appeals of the app. currently KNOV_DATA_PATH etc point at subfolders of the app's private internal storage (getFilesDir(), `/data/user/0/com.knov.wrapper/files/...`), which is sandboxed - no file manager app can browse it (even with "all files access"), only adb (`run-as`) or android studio's device file explorer. switching to external app-specific storage (getExternalFilesDir(), under `/storage/emulated/0/Android/data/com.knov.wrapper/files/...`) would be reachable over usb/mtp and by some file managers, though android 11+ still hides `Android/data/` from the stock Files app specifically - may need a different approach (e.g. SAF / a user-chosen folder) to be genuinely convenient
- [x] restrict WebView navigation to 127.0.0.1 and hand off other links to the system browser (MainActivity.kt) - rendered file content can contain links, and the default WebViewClient let the JS-enabled WebView navigate to arbitrary internet content
- [x] request POST_NOTIFICATIONS at runtime on API 33+ (MainActivity.kt) - without it the persistent notification (and its Stop action) silently never appeared, though the foreground service still ran fine
- [ ] `docker-build-android` uses `--no-cache`, so every android build redownloads/reinstalls the whole android sdk (cmdline-tools, platforms, build-tools) instead of reusing docker layer cache - matches the existing `--no-cache` convention on `docker-build-dev`/`docker-build-deployment` so left as-is for now, but worth revisiting since the sdk install layer is the most expensive part of this particular image

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

## analyze

**Role:** Act as a Senior Software Architect and Lead DevOps Engineer with 15+ years of experience in building scalable, production-grade systems. Also act as a pragmatic minimalist who strongly prefers the simplest solution that satisfies the current requirements.

**Task:** Analyze the uncommitted working-tree changes. Critique the implementation, check for architectural consistency, verify production readiness, and explicitly evaluate whether the solution is overengineered and can be simplified.

**Global Rules:**
- Prefer YAGNI, KISS, and boring, proven technology.
- Assume the simplest possible solution is preferred unless a more complex design is justified by a current, concrete requirement.
- Do not recommend abstractions, layers, patterns, dependencies, configuration, or indirection unless they solve a demonstrated problem today.
- For every recommendation, ask: Can this be removed, inlined, replaced by a framework/ORM/stdlib built-in, or done with less code?
- If complexity is justified, state exactly what current requirement justifies it.
- If the code is already appropriately simple, say so clearly. Do not invent complexity just to fill sections.

**Please provide your analysis in the following five sections:**

1. Implementation Review (The "Right Way")
- Is this the standard, idiomatic way to implement this feature in [Language/Framework]?
- Are there logical errors, edge cases, or security vulnerabilities (e.g., injection, race conditions, improper error handling) in this specific code block?
- Does it violate SOLID principles or common design patterns?
- If it follows SOLID/patterns but adds unnecessary indirection, call that out explicitly.

2. Architectural Consistency (The "Everywhere" Check)
- *Note: If I haven't provided enough context about the rest of the codebase, ask me for specific files to compare against.*
- Based on the code provided, does this follow a pattern that looks reusable and consistent?
- **Red Flags:** Does this introduce a "one-off" solution? (e.g., using a direct SQL query when the project uses an ORM, or hardcoding values that should be environment variables).
- Suggest how to refactor this to fit a unified architecture if it feels disjointed.
- If the existing architecture is already too complex, say that instead of forcing consistency with it.

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
- What can be deleted, inlined, hardcoded for now, replaced by a framework/ORM/stdlib feature, or deferred until actually needed?
- Is the added complexity justified by current scale, reliability, security, or team constraints? If not, simplify.
- Rank the simplifications by impact vs. effort.
- **Simplicity Verdict:** "Already simple," "Can be simplified," or "Significantly overengineered."

## review

Role: Act as a Staff-Level Software Engineer conducting a code review with a high bar for quality and maintainability.

Task: Review the current git diff. You are strictly prohibited from rewriting the code or providing "fixed" code snippets. You are only permitted to give your professional opinion on the changes.

Constraints:
- Do not output any code. Do not suggest code blocks, patches, or refactored versions of the provided diff.
- Verdict: If the changes are completely safe, logically sound, and meet standard best practices, explicitly state: "VERDICT: APPROVED" in your response.
- Problems: If you find any issues, do not fix them. Instead, explain why they are problematic, the potential impact (e.g., runtime error, security hole, performance bottleneck, unreadability), and optionally, the strategy to fix them (without writing the actual code).
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
- Side Effects: Are there changes to global state, environment variables, or external APIs that weren't considered?
- Architecture: Are the changes in line with the rest of the codebase?
- Ignore the i18n translations since they are unrelated.

Also give your opinion about the changes: is the current solution overengineered and can it be simplified?
