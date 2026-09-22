# knov android wrapper

Native shell app that runs the existing knov go server as a subprocess and shows it in a WebView -
an alternative to running the server through Termux. No JNI/gomobile bridge: the go binary is
unmodified, just spawned and killed like a normal process from a foreground service.

## build

From the repo root:

```
make mobile-apk
```

This cross-compiles the go server (`GOOS=android GOARCH=arm64`) into
`android/app/src/main/jniLibs/arm64-v8a/libknovserver.so`, then runs `./gradlew assembleDebug`.
Output: `android/app/build/outputs/apk/debug/app-debug.apk` - debug-signed only, which is fine for
sideloading (enable "install unknown apps"; this isn't published anywhere, no Play Store
involved).

Install/update it on a connected device with:

```
adb install -r android/app/build/outputs/apk/debug/app-debug.apk
```

Gradle needs a JDK it actually understands - the system `JAVA_HOME` may be too new. Set
`ANDROID_JAVA_HOME` to one that works (Android Studio's bundled JBR is a good option - see
`docs/developer.md`); it falls back to `JAVA_HOME` if unset (e.g. in CI via
`actions/setup-java`).

Alternatively, open `android/` directly in Android Studio (it will fetch the Gradle wrapper on
first sync) and build/run from there.

## how it works

- The go binary ships as a "native library" (`jniLibs/arm64-v8a/libknovserver.so`), not an asset:
  since Android 10, apps can't execute a file they write to their own data dir at runtime (W^X),
  even after `chmod +x`. Files under `nativeLibraryDir` are extracted executable at install time
  and are exempt from that restriction. `app/build.gradle.kts` sets
  `packaging { jniLibs { useLegacyPackaging = true } }` so it's actually extracted to disk instead
  of left mapped inside the APK.
- `ServerService` launches that binary with `ProcessBuilder`, pointing `KNOV_DATA_PATH` /
  `KNOV_STORAGE_PATH` / `KNOV_THEMES_PATH` / `KNOV_LOGS_PATH` / `KNOV_BACKUPS_PATH` at a `knov/`
  folder under shared storage (`dataRootDir()`) if "all files access" was granted, or subfolders of
  the app's private files dir otherwise (Android's storage sandbox blocks the paths knov defaults
  to). It runs as a foreground service (with a persistent notification, including a Stop action)
  so Android doesn't kill it in the background. If the process dies or fails to start, the service
  keeps retrying.
- On first launch (Android 11+ only), `MainActivity` asks once whether to grant
  `MANAGE_EXTERNAL_STORAGE` ("all files access") so the folders above land somewhere a normal file
  manager or USB/MTP can reach, instead of the app-private sandbox that only adb can see. This app
  is sideload-only (no Play Store distribution), so the policy restrictions around that permission
  don't apply. Declining (or devices below Android 11, where the permission doesn't exist) falls
  back to the private sandbox - reachable only via adb (`run-as`) or Android Studio's device file
  explorer. The in-app prompt itself is only ever shown once (reinstalling or clearing app data is
  the only way to see it again) - but the permission it asks for is a normal Android one, changeable
  anytime from Settings => Apps => Knov => Permissions, with no reinstall needed. `dataRootDir()`
  re-checks it on every server (re)start, so toggling it in Settings and then hitting "Save &
  restart" in the `.env` editor (even without editing anything) is enough to apply a change.
  Switching does not move existing data between the two locations - whichever one is active after
  the restart is what the server sees, so anything already saved under the other one is effectively
  hidden until moved over by hand.
- `MainActivity` starts that service, shows a plain "starting server..." message while polling
  `http://127.0.0.1:1324/`, then loads it in a `WebView` once it responds. Tapping the persistent
  notification reopens this activity.
- The WebView shows the same knov web UI as desktop, including its logs page - there's no
  separate native log viewer. From the app, reach it the same way as on desktop: Help > Admin >
  Logs (`/system/logs`).
- A small ".env" button opens a plain text editor for the app's private `.env` file (the go binary
  already applies any `.env` in its working directory onto the process environment on startup, but
  there was previously no way to get a file into the app's private storage without adb). Saving
  restarts the server so the new values take effect.

## known gaps / not yet done

- No app icon polish beyond a basic one generated from `static/knov_logo.png`, no dedicated launch
  screen.
- No handling of Android battery optimization killing the foreground service on some OEMs - not
  yet tested on a real device over time.

## signing

`app/debug.keystore` is checked in and used for the debug build's signing config (password
`android`, alias `androiddebugkey` - the standard debug-keystore defaults, not a real secret).
Without this, gradle's own ephemeral per-machine debug keystore would give every CI-built release
a different signature, breaking `adb install -r` upgrades across releases. Debug-signed only - if
this ever needs a real release keystore (e.g. for wider distribution), that's a separate signing
config fed from a GitHub Actions secret, not this file.
