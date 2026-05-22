# small stuff

- media.go - determine filetype for metadata
- new theme like files.md with a sidebar on the right side which shows the file like a file browser and on top of the sidebar the action icons: settings, admin...
- overview - browsefiles doesnt work anymore
- overview: show folder tree instead of everything in a single level
- make rebuild metadata concurrent e.g. each letter?
- in references sidebar add - add button, remove button and edit button to header?
- why is the metadata for all files retrieved if i only want to rebuild the links for one file (MetaDataLinksRebuildForFile)
- fix compact and reader view
- table paginate/search/filter is not working (handleAPIGetTable is not used??)
- load all files from cache?
  - http://localhost:1324/browse/folders/systemadmin is running through all files?
- dont run the search indexing at startup or slightly delayed so the app starts
- codeblocks - copy button
- codeblocks setting - wrap content
- .png, .svg, .jpg ... are hardcoded (render_media.go, server.go) - refactor to use IsImageExtension/mime package in settings.go
- make certain settings required in thememanager
- Dashboard
  - make the positions work with a custom layout work
  - Add widget drag & drop reordering
  - Implement widget resizing
  - Add dashboard export/import
- export metadata to yaml header in markdown files
- performance updates
  - use Query() instead of a loop through files.GetAllFiles()
  - use Query in filter.go
  - Refactor filter.go to use query
- create filter manually not per ai..
- migration solution
metadataStorage_sqlite - migration
// migrate: add references column if missing
if _, err := ss.db.Exec(`ALTER TABLE metadata ADD COLUMN references TEXT`); err != nil {
	// column likely already exists, ignore
	logging.LogDebug("references column already exists or migration skipped: %v", err)
})


# concurrent

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

# daily

single source of truth is the metadata - we just display it differently
one "chat" like files.md => each line can be added to another file
