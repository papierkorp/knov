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
- translations
- add a `create_your_own_theme.md` file

**per ai**
- move all scripts into files (browser inspector)
- media gitignore
- add chat to a single file?
- references - add date
- detect if a request is already loading and block multiple requests of the same kind
- rename testdata to test and add some kind of test (playwright?)
- subediting for `Themes & Appearance` not working
- add quill editor
- editor settings (e.g. quill no toolbar, default view for toastui)
- include the editor/static javascript per thememanager?
- wikilink feature problem: if i scroll through all available files with the arrow keys it doesnt scroll down (the marker is moving down but i stay at the same visible files at the top)

- deployment
  - codeberg deployment pipeline?
  - make docker build viable
    - for usage
    - for devs
- backup solution
- fix all the editor warnings

# performance updates

**read**
- use Query() instead of a loop through files.GetAllFiles()
- use Query in filter.go
- Refactor filter.go to use query

**write**
- add a SetBatch for rebuild metadata
- But if you ever wanted to make it faster the right approach would be to build a one-time index of deleted files at startup/cronjob time and search that instead of walking commits live. (git history search)

**concurrent**
- make rebuild metadata concurrent e.g. each letter?

dont give me any code yet - what options do i ave to make the rebuild metadata concurrent?
at the moment im running through ALL files mutliple times (e.g. with MetaDataPurgeStale, MetaDataPurgeDuplicates, linksrebuild..)May 13Claude responded: Looking at the current flow, you have roughly:Weighed concurrent metadata rebuild strategies to eliminate redundant file iterationsWeighed concurrent metadata rebuild strategies to eliminate redundant file iterationsLooking at the current flow, you have roughly:

Walk all files → build/update metadata (sequential)
Walk all metadata → purge stale (sequential)
Walk all metadata → purge duplicates (sequential)
Walk all metadata → rebuild links (sequential)

Options:
A — Single pass, pipeline everything
One walk over all files, do metadata update + link extraction in the same iteration. Then one walk over metadataStorage for purge stale + duplicates combined. Cuts the number of full scans from ~4 to 2. No concurrency needed, just smarter ordering.
B — Worker pool per file
Keep the multi-pass structure but process each file concurrently using a sync.WaitGroup + buffered channel as a worker pool. Most useful for the rebuild step since it does I/O (read file → extract links → write metadata). Stale/duplicate purges are already just map operations so they're fast.
C — Fan-out with errgroup
Use golang.org/x/sync/errgroup with a semaphore to bound concurrency. Cleaner error handling than raw goroutines. Same idea as B but more idiomatic.
D — Combine A + C
Single pass over files with a worker pool, doing metadata + link extraction per file concurrently, then one combined purge pass at the end. Best of both worlds.
My recommendation: D. The real bottleneck is the per-file I/O in the rebuild (reading file content for link/title extraction). Parallelizing that with a bounded worker pool (e.g. runtime.NumCPU() workers) gives the biggest win. The purge passes are already fast (just map lookups) so combining them into one pass is enough there — no need to parallelize.
The main thing to watch out for: metadataStorage writes need a mutex or you need to collect results and write sequentially after the workers finish.
Want to go with D?

# docs

small, precise and concise, high level overview, no examples that are prone to change, just a few bullet points
