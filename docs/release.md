# Release information

The page inside the app (**/system/release**) shows details about the exact version you are running right now. Those details depend on your copy of the app, so they are not written into this file.

Below the details you will find the release notes: one entry per version, newest at the top. Look for the version you are on, or the one you want to update to, to see what changed.

If there are no version entries yet, no official release has been published so far. In the meantime the [changelog](/system/changelog) lists every change.

## How to update

Stop the app, replace the program file with the new version, and start it again. Your notes and settings stay where they are and are not touched. It is still a good idea to make a backup first on the **/system/backup** page.

Look at the breaking changes in the release notes below to see if anything relevant changed before upgrading.

## What the details mean

- **Version** - the release name, like `1.0.0`. `dev` means you are running an unreleased build.
- **Build** - a unique fingerprint of this exact build. Handy to quote when reporting a problem. `-dev` at the end means it was not an official release build.
- **Build time** - when this build was created.
- **Go version** - the version of the tool the app was built with.
- **OS / Arch** - the operating system and processor type this build is for.
- **Last commit** - a short description of the most recent change in this build.
