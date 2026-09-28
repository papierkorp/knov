// Package kanban provides business logic for the kanban board feature.
package kanban

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/kanbanStorage"
	"knov/internal/logging"
	"knov/internal/markdown"
	"knov/internal/parser"
	"knov/internal/pathutils"
)

// Card holds the data for a single kanban card.
type Card struct {
	FilePath      string
	Title         string
	Collection    string
	Status        string
	Tags          []string
	CreatedAt     string
	LastEdited    string
	KanbanAddedAt string
	KanbanMovedAt string
	Excerpt       string
}

// ExcerptRunes is the number of body-text runes shown on a kanban card.
const ExcerptRunes = 30

// Column holds a status column and its ordered cards.
type Column struct {
	Status string
	Cards  []Card
}

// SortBy defines how cards within each column are ordered.
type SortBy string

const (
	SortCreatedAt     SortBy = "createdAt"     // oldest first
	SortEditedAt      SortBy = "editedAt"      // most recently edited first
	SortAlphabetical  SortBy = "alphabetical"  // by title A→Z
	SortSize          SortBy = "size"          // smallest first
	SortKanbanAddedAt SortBy = "kanbanAddedAt" // most recently added to kanban first
	SortKanbanMovedAt SortBy = "kanbanMovedAt" // most recently moved on kanban first
)

// folderMatches reports whether meta's directory is folderPath itself or a subfolder of it,
// i.e. board scoping is recursive: a board configured for "projects/work" also includes
// "projects/work/urgent/*".
func folderMatches(meta *files.Metadata, folderPath string) bool {
	return pathutils.FolderContains(strings.Join(meta.Folders, "/"), folderPath)
}

// resolveBoardFolder returns the most specific (longest) configured kanban board folder that
// contains dir, or "" if no configured board covers it.
func resolveBoardFolder(dir string) string {
	best := ""
	for _, b := range configmanager.GetKanbanBoards() {
		if pathutils.FolderContains(dir, b.FolderPath) && len(b.FolderPath) > len(best) {
			best = b.FolderPath
		}
	}
	return best
}

// BuildBoard runs the filter, applies optional search, sorts by sortBy, and returns columns with cards.
// folderPath scopes the board to that folder and its subfolders. Only cfg's criteria and logic are
// used - a board always shows every matching card, so cfg.Limit is deliberately ignored.
func BuildBoard(folderPath string, cfg *filter.Config, searchQuery string, sortBy SortBy) ([]Column, error) {
	columns := configmanager.GetKanbanColumns()
	prefix := configmanager.GetKanbanPrefix()

	// scope to the board's own cards first, so the filter criteria only ever run
	// over the files that can actually land on this board instead of every file
	candidates, err := cardFilesInFolder(folderPath)
	if err != nil {
		logging.LogError(logging.KeyApp, "kanban: failed to collect cards for folder %s: %v", folderPath, err)
		candidates = nil
	}
	var matched []files.File
	if cfg != nil {
		matched = filter.FilterFileList(candidates, cfg.Criteria, cfg.Logic)
	} else {
		matched = candidates
	}

	cardsByStatus := make(map[string][]Card, len(columns))
	for _, col := range columns {
		cardsByStatus[col] = []Card{}
	}

	lq := strings.ToLower(searchQuery)
	for _, file := range matched {
		meta := file.Metadata

		status := StatusFromTags(meta.Tags, prefix)
		if status == "" || !slices.Contains(columns, status) {
			continue
		}

		if lq != "" {
			title := strings.ToLower(meta.Title)
			fp := strings.ToLower(file.Path)
			if !strings.Contains(title, lq) && !strings.Contains(fp, lq) {
				continue
			}
		}

		cardsByStatus[status] = append(cardsByStatus[status], cardFromFile(file, status))
	}

	// precompute file sizes once if needed
	var fileSizes map[string]int64
	if sortBy == SortSize {
		fileSizes = make(map[string]int64, len(matched))
		for col := range cardsByStatus {
			for _, c := range cardsByStatus[col] {
				if fi, err := os.Stat(pathutils.ToDocsPath(c.FilePath)); err == nil {
					fileSizes[c.FilePath] = fi.Size()
				}
			}
		}
	}

	// only load stored order for custom sort
	var storedOrder Order
	if sortBy == "" {
		storedOrder, _ = GetOrder(folderPath)
	}

	for col, cards := range cardsByStatus {
		switch sortBy {
		case "":
			// baseline: createdAt (newest first), then apply drag-drop order on top
			sort.Slice(cards, func(i, j int) bool {
				return cards[i].CreatedAt > cards[j].CreatedAt
			})
			if storedPaths, ok := storedOrder[col]; ok {
				paths := make([]string, len(cards))
				for i, c := range cards {
					paths[i] = c.FilePath
				}
				ordered := ApplyOrder(storedPaths, paths)
				posMap := make(map[string]int, len(ordered))
				for i, fp := range ordered {
					posMap[fp] = i
				}
				sort.SliceStable(cards, func(i, j int) bool {
					return posMap[cards[i].FilePath] < posMap[cards[j].FilePath]
				})
			}
		case SortCreatedAt:
			sort.Slice(cards, func(i, j int) bool {
				return cards[i].CreatedAt < cards[j].CreatedAt
			})
		case SortEditedAt:
			sort.Slice(cards, func(i, j int) bool {
				return cards[i].LastEdited > cards[j].LastEdited // most recent first
			})
		case SortKanbanAddedAt:
			sort.Slice(cards, func(i, j int) bool {
				// empty string sorts last (cards never added via kanban)
				if cards[i].KanbanAddedAt == cards[j].KanbanAddedAt {
					return false
				}
				if cards[i].KanbanAddedAt == "" {
					return false
				}
				if cards[j].KanbanAddedAt == "" {
					return true
				}
				return cards[i].KanbanAddedAt > cards[j].KanbanAddedAt // most recent first
			})
		case SortKanbanMovedAt:
			sort.Slice(cards, func(i, j int) bool {
				// empty string sorts last (cards never moved via kanban)
				if cards[i].KanbanMovedAt == cards[j].KanbanMovedAt {
					return false
				}
				if cards[i].KanbanMovedAt == "" {
					return false
				}
				if cards[j].KanbanMovedAt == "" {
					return true
				}
				return cards[i].KanbanMovedAt > cards[j].KanbanMovedAt // most recent first
			})
		case SortAlphabetical:
			sort.Slice(cards, func(i, j int) bool {
				return strings.ToLower(cards[i].Title) < strings.ToLower(cards[j].Title)
			})
		case SortSize:
			sort.Slice(cards, func(i, j int) bool {
				return fileSizes[cards[i].FilePath] < fileSizes[cards[j].FilePath]
			})
		}
		cardsByStatus[col] = cards
	}

	cols := make([]Column, 0, len(columns))
	for _, col := range columns {
		cols = append(cols, Column{Status: col, Cards: cardsByStatus[col]})
	}
	return cols, nil
}

// MoveCard updates the kanban status tag on a file and returns the previous status (empty if
// none) and the file's current path - unchanged unless the resolved board has foldersync
// enabled (via the Folder Sync setting), in which case the file is physically moved into
// board/newStatus/ and newFilePath reflects that. The physical move (if
// any) always happens before the tag is touched: if it fails outright, MoveCard returns an error
// and nothing changes, so the tag can never end up claiming a location the file was never
// actually moved to. boardFolder scopes the event log entry (and, under foldersync, the move
// target); pass "" to fall back to guessing the board from the file's own location (used when
// the caller doesn't know which board triggered the move).
func MoveCard(boardFolder, filePath, newStatus string) (oldStatus, newFilePath string, err error) {
	normalizedPath := pathutils.ToWithPrefix(filePath)
	newFilePath = filePath

	// unlocked read, only to learn the file's current folder (to decide whether a physical
	// move is needed and which board it belongs to) - same narrow race window already accepted
	// by moveFileMetadata for the move below, see its docstring; here it can also make the
	// physical move below fail with ErrMoveSourceMissing under a concurrent mover of the same
	// path, which is an accepted, fail-safe race (returns an error, changes nothing) rather than
	// one worth retrying - see prior review discussion for why a retry can't actually recover it
	meta, err := files.MetaDataGet(normalizedPath)
	if err != nil || meta == nil {
		return "", filePath, err
	}

	dir := strings.Join(meta.Folders, "/")
	board := boardFolder
	if board == "" || !pathutils.FolderContains(dir, board) {
		// caller either didn't say which board, or named one the file isn't actually under -
		// never trust an unvalidated board hint for the physical move below, only for scoping
		// the event log entry
		board = resolveBoardFolder(dir)
	}
	eventFolder := board
	if eventFolder == "" {
		eventFolder = dir // file isn't under any configured board; log under its own folder
	}

	// foldersync: physically relocate the card into board/newStatus/ so its on-disk location
	// mirrors the tag - only for boards that opted in via the Folder Sync setting. Done before the
	// tag write below so a failed move can't leave the tag out of sync with where the file
	// actually is.
	if cfgBoard, ok := configmanager.GetKanbanBoardByFolder(board); ok && cfgBoard.FolderSync {
		targetDir := board + "/" + newStatus
		if targetDir != dir {
			targetPath, moveErr := moveFileUnique(logging.KeyApp, filePath, targetDir, filepath.Base(filePath))
			if moveErr != nil && !errors.Is(moveErr, files.ErrLinkUpdateFailed) {
				// the rename itself never happened - filePath is still where it was, so bail out
				// before the tag is touched instead of applying a status that claims a location
				// the file was never actually moved to
				return "", filePath, fmt.Errorf("kanban: foldersync failed to move %s to %s: %w", filePath, targetPath, moveErr)
			}
			if moveErr != nil {
				// the rename itself already succeeded on disk and only the metadata/link fixup
				// afterward had trouble - the file really is at targetPath now, so state must
				// reflect that instead of pointing at a path that no longer exists
				logging.LogWarning(logging.KeyApp, "kanban: foldersync moved %s to %s but link update failed: %v", filePath, targetPath, moveErr)
			}
			newFilePath = targetPath
		}
	}

	normalizedNewPath := pathutils.ToWithPrefix(newFilePath)
	var found bool

	// MetaDataMutate holds the path's write lock across this whole read-modify-write, so a
	// concurrent writer of the same file (e.g. file-sync's per-changed-file metadata
	// refresh) can't read stale tags in between and silently revert this move on its own
	// save.
	err = files.MetaDataMutate(normalizedNewPath, func(meta *files.Metadata, existed bool) (bool, error) {
		if !existed {
			return false, nil
		}
		found = true
		oldStatus = applyStatusTag(meta, newStatus)

		now := time.Now()
		if meta.KanbanAddedAt.IsZero() {
			meta.KanbanAddedAt = now
		}
		meta.KanbanMovedAt = now
		return true, nil
	})
	if err != nil || !found {
		return "", newFilePath, err
	}

	files.RefreshCaches()

	if err := kanbanStorage.LogEvent(newFilePath, eventFolder, oldStatus, newStatus); err != nil {
		logging.LogWarning(logging.KeyApp, "kanban: failed to log event for %s: %v", newFilePath, err)
	}
	logging.LogInfo(logging.KeyApp, "kanban: moved card %s to status %s", newFilePath, newStatus)
	return oldStatus, newFilePath, nil
}

// moveFileUnique moves oldPath into dir/name, or dir/name_2, dir/name_3, ... (inserted before the
// extension) if that candidate is already taken - so foldersync never fails a move just because
// another card, coming from a different subfolder, already has the same filename in the
// destination status folder. Unlike a stat-then-move check, each candidate is tried by actually
// attempting the move and reacting to files.ErrMoveTargetExists, which is decided under
// movePhysical's lock - so two callers racing for the same name can't both pass a check that's
// already stale by the time they act on it; the loser simply retries the next candidate. Caps
// out at maxMoveUniqueAttempts as a safety net against an unbounded loop if MoveFileNoRefresh
// ever misreports ErrMoveTargetExists.
const maxMoveUniqueAttempts = 100

func moveFileUnique(key logging.Key, oldPath, dir, name string) (newPath string, err error) {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	candidate := dir + "/" + name
	for n := 2; n <= maxMoveUniqueAttempts; n++ {
		err = files.MoveFileNoRefresh(key, oldPath, candidate)
		if err == nil || !errors.Is(err, files.ErrMoveTargetExists) {
			return candidate, err
		}
		candidate = fmt.Sprintf("%s/%s_%d%s", dir, base, n, ext)
	}
	return candidate, fmt.Errorf("too many filename collisions in %s", dir)
}

// applyStatusTag replaces meta's kanban status tag with newStatus, returning the previous
// status (or "" if none was set). Shared by MoveCard's tag→folder sync and SyncFolderTag's
// folder→tag sync so both directions agree on what "the" kanban tag is.
func applyStatusTag(meta *files.Metadata, newStatus string) (oldStatus string) {
	oldStatus = StatusFromTags(meta.Tags, configmanager.GetKanbanPrefix())
	newTag := configmanager.KanbanStatusTag(newStatus)
	filtered := meta.Tags[:0:0]
	for _, t := range meta.Tags {
		if !configmanager.IsKanbanTag(t) {
			filtered = append(filtered, t)
		}
	}
	meta.Tags = append(filtered, newTag)
	return oldStatus
}

// SyncFolderTag is foldersync's reverse direction: called by the file-sync cronjob for a file
// it just found on disk (newly created, edited, or renamed), it checks whether path sits
// directly inside one of its board's status folders and, if so, updates the file's kanban tag to
// match - covering a plain "drop a new file into board/status/" just as much as an actual move.
// changedAt is when the physical change happened (the git commit's authored time) - if the tag
// was changed more recently than that, the metadata change wins and the folder hint is ignored,
// so an in-app drag-drop (which already moved the file itself) can't be undone by the cronjob
// picking up its own rename on the next run.
//
// KanbanMovedAt only advances on an in-app move (MoveCard), so that's the only kind of tag
// change this protects against the folder hint. A tag edited by hand (or by anything else that
// bypasses MoveCard) without also moving the file leaves it sitting in a folder that disagrees
// with its tag - once foldersync is on for the board, the folder is treated as authoritative and
// wins on the next sync. That's intentional, not a race: foldersync's contract is that folder
// and tag agree, so a hand-edited tag that disagrees with the current folder is describing an
// inconsistent state, not a competing source of truth.
func SyncFolderTag(path string, changedAt time.Time) error {
	normalizedPath := pathutils.ToWithPrefix(path)

	var oldStatus, status, board string
	var applied bool
	err := files.MetaDataMutate(normalizedPath, func(meta *files.Metadata, existed bool) (bool, error) {
		if !existed {
			return false, nil
		}
		dir := strings.Join(meta.Folders, "/")
		board = resolveBoardFolder(dir)
		if board == "" {
			return false, nil // not under any configured board
		}
		if cfgBoard, ok := configmanager.GetKanbanBoardByFolder(board); !ok || !cfgBoard.FolderSync {
			return false, nil // foldersync not enabled for this board
		}
		status = strings.TrimPrefix(dir, board+"/")
		if status == dir || !slices.Contains(configmanager.GetKanbanStatuses(), status) {
			return false, nil // not directly inside a status folder
		}

		oldStatus = StatusFromTags(meta.Tags, configmanager.GetKanbanPrefix())
		if oldStatus == status {
			// already correct - this is what stops an in-app MoveCard from being undone by the
			// cronjob picking up the physical move it just made: MoveCard sets the tag and moves
			// the file into the matching status folder in the same call, so by the time this
			// runs both already agree and there's nothing to sync. The changedAt check below is
			// only a secondary guard for the remaining case (a file edited/created directly in a
			// status folder, no matching MoveCard call, and its tag still lagging).
			return false, nil
		}
		if !meta.KanbanMovedAt.IsZero() && meta.KanbanMovedAt.After(changedAt) {
			return false, nil // tag was changed after this physical change - metadata wins
		}

		applyStatusTag(meta, status)
		if meta.KanbanAddedAt.IsZero() {
			meta.KanbanAddedAt = changedAt
		}
		meta.KanbanMovedAt = changedAt
		applied = true
		return true, nil
	})
	if err != nil || !applied {
		return err
	}

	relPath := pathutils.ToRelative(path)
	if err := kanbanStorage.LogEvent(relPath, board, oldStatus, status); err != nil {
		logging.LogWarning(logging.KeyFileSync, "kanban: failed to log foldersync event for %s: %v", relPath, err)
	}
	logging.LogInfo(logging.KeyFileSync, "kanban: foldersync set status %s for %s from physical change", status, relPath)
	return nil
}

// GetEvents returns kanban move events with optional filters, newest first.
// Pass empty strings / nil times to skip those filters; limit=0 means no limit.
func GetEvents(folderPath, filePath string, from, to *time.Time, limit int) ([]kanbanStorage.Event, error) {
	return kanbanStorage.GetEvents(folderPath, filePath, from, to, limit)
}

// cardFromFile builds a Card from a cached file entry and its already-resolved kanban status.
func cardFromFile(file files.File, status string) Card {
	meta := file.Metadata
	relPath := pathutils.ToRelative(file.Path)
	card := Card{
		FilePath:   relPath,
		Title:      meta.Title,
		Collection: meta.Collection,
		Status:     status,
		Tags:       meta.Tags,
		CreatedAt:  meta.CreatedAt.Format("2006-01-02"),
		LastEdited: meta.LastEdited.Format("2006-01-02"),
		Excerpt:    Excerpt(pathutils.ToDocsPath(relPath), ExcerptRunes),
	}
	if !meta.KanbanAddedAt.IsZero() {
		card.KanbanAddedAt = meta.KanbanAddedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if !meta.KanbanMovedAt.IsZero() {
		card.KanbanMovedAt = meta.KanbanMovedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return card
}

// Archived returns kanban cards with the archive status for the given folder (and its
// subfolders), newest-created first. A direct scan scoped by folder + status tag, same
// approach as cardFilesInFolder, rather than the general filter engine.
func Archived(folderPath string) ([]Card, error) {
	archiveStatus := configmanager.GetKanbanArchiveStatus()
	prefix := configmanager.GetKanbanPrefix()

	allFiles, err := files.GetAllFilesCached()
	if err != nil {
		return nil, err
	}
	allFiles = files.FilterByVisibility(allFiles, configmanager.HideScopeKanban)

	var cards []Card
	for _, file := range allFiles {
		if file.Metadata == nil || !folderMatches(file.Metadata, folderPath) {
			continue
		}
		if StatusFromTags(file.Metadata.Tags, prefix) != archiveStatus {
			continue
		}
		cards = append(cards, cardFromFile(file, archiveStatus))
	}

	sort.Slice(cards, func(i, j int) bool {
		return cards[i].CreatedAt > cards[j].CreatedAt
	})
	return cards, nil
}

// FilterAncestorsByAllowedStatus drops ancestors whose descendant cards in folderPath (and its
// subfolders) have no allowed status configured via the Ancestor Filter Statuses setting - e.g.
// hides an "epic" once every child card under it has been archived. No allowed status configured
// means no filtering (all ancestors kept).
func FilterAncestorsByAllowedStatus(ancestors []string, folderPath string) ([]string, error) {
	allowed := configmanager.GetKanbanAncestorAllowedStatus()
	if len(allowed) == 0 {
		return ancestors, nil
	}

	prefix := configmanager.GetKanbanPrefix()
	allFiles, err := files.GetAllFilesCached()
	if err != nil {
		return nil, err
	}

	active := make(map[string]struct{})
	for _, file := range allFiles {
		if file.Metadata == nil || len(file.Metadata.Ancestor) == 0 || !folderMatches(file.Metadata, folderPath) {
			continue
		}
		status := StatusFromTags(file.Metadata.Tags, prefix)
		if status == "" || !slices.Contains(allowed, status) {
			continue
		}
		active[file.Metadata.Ancestor[0]] = struct{}{}
	}

	var result []string
	for _, a := range ancestors {
		if _, ok := active[a]; ok {
			result = append(result, a)
		}
	}
	return result, nil
}

// cardFilesInFolder returns all cached files that are kanban cards (have a kanban status tag)
// scoped to folderPath (and its subfolders), leaving out files hidden for the kanban scope.
func cardFilesInFolder(folderPath string) ([]files.File, error) {
	prefix := configmanager.GetKanbanPrefix()
	allFiles, err := files.GetAllFilesCached()
	if err != nil {
		return nil, err
	}

	var cards []files.File
	for _, file := range allFiles {
		if file.Metadata == nil || !folderMatches(file.Metadata, folderPath) {
			continue
		}
		if StatusFromTags(file.Metadata.Tags, prefix) == "" {
			continue
		}
		cards = append(cards, file)
	}
	return files.FilterByVisibility(cards, configmanager.HideScopeKanban), nil
}

// TagsForFolder returns all unique non-kanban tags present on kanban cards in the folder (and its subfolders).
func TagsForFolder(folderPath string) ([]string, error) {
	cards, err := cardFilesInFolder(folderPath)
	if err != nil {
		return nil, err
	}

	tagSet := make(map[string]struct{})
	for _, file := range cards {
		for _, t := range file.Metadata.Tags {
			if !configmanager.IsKanbanTag(t) {
				tagSet[t] = struct{}{}
			}
		}
	}

	tags := make([]string, 0, len(tagSet))
	for t := range tagSet {
		tags = append(tags, t)
	}
	slices.Sort(tags)
	return tags, nil
}

// FilesForFolder returns the file paths of all kanban cards (files with a kanban status) in the folder
// (and its subfolders), sorted.
func FilesForFolder(folderPath string) ([]string, error) {
	cards, err := cardFilesInFolder(folderPath)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(cards))
	for _, file := range cards {
		paths = append(paths, pathutils.ToRelative(file.Path))
	}
	slices.Sort(paths)
	return paths, nil
}

// Excerpt returns the first maxRunes runes of meaningful body text from a file,
// stripping front matter and common markdown syntax.
func Excerpt(fullPath string, maxRunes int) string {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return ""
	}

	body := parser.StripFrontMatter(data)
	lines := strings.Split(string(body), "\n")
	inFence := markdown.FenceMask(lines)
	for i, line := range lines {
		if inFence[i] {
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") {
			continue
		}
		line = strings.NewReplacer("**", "", "__", "", "*", "", "_", "").Replace(line)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if utf8.RuneCountInString(line) <= maxRunes {
			return line
		}
		return string([]rune(line)[:maxRunes]) + "…"
	}
	return ""
}

// StatusFromTags extracts the kanban status value from a tag list; returns "" if absent.
func StatusFromTags(tags []string, prefix string) string {
	statusPrefix := prefix + "-status-"
	for _, t := range tags {
		if strings.HasPrefix(t, statusPrefix) {
			return strings.TrimPrefix(t, statusPrefix)
		}
	}
	return ""
}

// TagFromList returns the first kanban tag found in the list, or "".
func TagFromList(tags []string) string {
	prefix := configmanager.GetKanbanPrefix() + "-"
	for _, t := range tags {
		if strings.HasPrefix(t, prefix) {
			return t
		}
	}
	return ""
}

// TagNotifyMsg returns a human-readable message for a kanban tag change, or "" when unchanged.
func TagNotifyMsg(oldTag, newTag string) string {
	switch {
	case oldTag == "" && newTag != "":
		return "kanban tag added: " + newTag
	case oldTag != "" && newTag != "" && oldTag != newTag:
		return "kanban status changed: " + oldTag + " → " + newTag
	default:
		return ""
	}
}

// ConfigWarnings lists inconsistencies in the current kanban settings that don't block a save:
// an empty status list, board folders that don't exist (e.g. renamed), columns, archive or ancestor filter statuses
// that aren't in the status list, folder sync folders without a board, auto-create tags under
// the prefix that aren't valid status tags. It only reads settings (plus one stat per board), no
// file scan, since it runs after every kanban setting save.
func ConfigWarnings(t func(string, ...any) string) []string {
	prefix, statuses := configmanager.GetKanbanPrefix(), configmanager.GetKanbanStatuses()
	var warnings []string
	if len(statuses) == 0 {
		warnings = append(warnings, t("the status list is empty, no card can be placed on a board"))
	}
	boards := configmanager.GetKanbanBoards()
	for _, b := range boards {
		if BoardFolderMissing(b.FolderPath) {
			warnings = append(warnings, t("board folder %q doesn't exist", b.FolderPath))
		}
	}
	for _, c := range configmanager.GetKanbanColumns() {
		if !slices.Contains(statuses, c) {
			warnings = append(warnings, t("column %q is not in the status list", c))
		}
	}
	if a := configmanager.GetKanbanArchiveStatus(); a != "" && !slices.Contains(statuses, a) {
		warnings = append(warnings, t("archive status %q is not in the status list", a))
	}
	for _, a := range configmanager.GetKanbanAncestorAllowedStatus() {
		if !slices.Contains(statuses, a) {
			warnings = append(warnings, t("ancestor filter status %q is not in the status list", a))
		}
	}
	for _, f := range configmanager.KanbanFolderSync.Get() {
		if !slices.ContainsFunc(boards, func(b configmanager.KanbanBoard) bool { return b.FolderPath == configmanager.NormalizeKanbanFolder(f) }) {
			warnings = append(warnings, t("folder sync folder %q has no matching board", f))
		}
	}
	for _, a := range configmanager.GetAutoCreateTags() {
		status, isStatus := strings.CutPrefix(a.Tag, prefix+"-status-")
		if strings.HasPrefix(a.Tag, prefix+"-") && !(isStatus && slices.Contains(statuses, status)) {
			warnings = append(warnings, t("auto-create tag %q (KNOV_AUTOCREATE_TAGS) doesn't match the kanban prefix/statuses", a.Tag))
		}
	}
	return warnings
}

// BoardFolderMissing reports whether a configured board's folder doesn't exist (e.g. renamed).
func BoardFolderMissing(folderPath string) bool {
	_, err := os.Stat(pathutils.ToDocsPath(folderPath))
	return os.IsNotExist(err)
}
