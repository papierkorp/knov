# small stuff

**manual**
- add a smoketest todo file to testfiles
- add tags, parent and references to testfiles
- create filter manually not per ai..

**per ai**
- move media to browse
- chat: select multiple + move/delete multiple
- codeblocks - copy button
- codeblocks setting - wrap content

**long term**
- i want to get rid of the dokuwiki file support (i want to keep the dokuwiki to markdown export) but i dont want to display dokuwiki files any longer
- media.go - determine filetype for metadata
- fix windows path issues
- /releasenotes route with new icon on the bottom of the rail theme
- dont run the search indexing at startup or slightly delayed so the app starts
- Dashboard
  - make the positions work with a custom layout work
  - Add widget drag & drop reordering
  - Implement widget resizing
  - Add dashboard export/import
- export metadata to yaml header in markdown files

# performance updates

- use Query() instead of a loop through files.GetAllFiles()
- use Query in filter.go
- Refactor filter.go to use query

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

# kanban

/kanban route
use tags as key:value pairs?
stati:
- inbox/backlog
- in progress
- archive
- blocked
