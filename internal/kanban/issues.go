package kanban

import (
	"slices"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/kanbanStorage"
	"knov/internal/logging"
	"knov/internal/pathutils"
)

// IssueKind names a kanban tag inconsistency found by ScanIssues.
type IssueKind string

const (
	IssueMultipleStatus IssueKind = "multiple-status" // more than one status tag
	IssueUnknownStatus  IssueKind = "unknown-status"  // status not in the status list
	IssueUnknownTag     IssueKind = "unknown-tag"     // <prefix>-<x> tag outside the status namespace
	IssueFolderMismatch IssueKind = "folder-mismatch" // foldersync board: status folder and tag disagree
	IssueNoBoard        IssueKind = "no-board"        // status tag, but no board covers the file's folder
)

// Issue is a kanban tag inconsistency of one file. Fix is the status CleanupIssues sets - empty
// when the issue is only reported.
type Issue struct {
	Path  string    `json:"path"`
	Board string    `json:"board,omitempty"`
	Kind  IssueKind `json:"kind"`
	Tags  []string  `json:"tags"`
	Fix   string    `json:"fix,omitempty"`
}

// CleanupResult is the outcome of a CleanupIssues run.
type CleanupResult struct {
	Fixed  int `json:"fixed"`
	Failed int `json:"failed"`
}

// ScanIssues lists the kanban tag inconsistencies of every file without changing anything.
func ScanIssues() ([]Issue, error) {
	allFiles, err := files.GetAllFilesCached()
	if err != nil {
		return nil, err
	}
	issues := []Issue{} // an empty scan is [], not null
	for _, f := range allFiles {
		if f.Metadata != nil {
			issues = append(issues, fileIssues(pathutils.ToRelative(f.Path), strings.Join(f.Metadata.Folders, "/"), f.Metadata.Tags)...)
		}
	}
	return issues, nil
}

// fileIssues returns the issues of a single file. On a foldersync board the status folder wins
// (same contract as SyncFolderTag), otherwise several status tags are reduced to the first valid
// one - the column the board shows when the first status tag is valid. Unknown statuses/tags and
// cards outside any board are only reported.
func fileIssues(path, dir string, tags []string) []Issue {
	prefix, statuses := configmanager.GetKanbanPrefix(), configmanager.GetKanbanStatuses()
	var found, statusTags, unknownTags []string
	for _, t := range tags {
		if s, ok := strings.CutPrefix(t, prefix+"-status-"); ok {
			found = append(found, s)
			statusTags = append(statusTags, t)
		} else if strings.HasPrefix(t, prefix+"-") {
			unknownTags = append(unknownTags, t)
		}
	}

	var issues []Issue
	if len(unknownTags) > 0 {
		issues = append(issues, Issue{Path: path, Kind: IssueUnknownTag, Tags: unknownTags})
	}

	board := resolveBoard(dir)
	folderStatus := statusFolder(board, dir)

	issue := Issue{Path: path, Board: board.FolderPath, Tags: statusTags}
	switch {
	case folderStatus != "" && !slices.Equal(found, []string{folderStatus}):
		issue.Kind, issue.Fix = IssueFolderMismatch, folderStatus
	case len(found) == 0:
		return issues
	case board.FolderPath == "":
		issue.Kind = IssueNoBoard
	case len(found) > 1:
		issue.Kind = IssueMultipleStatus
		for _, s := range found {
			if slices.Contains(statuses, s) {
				issue.Fix = s
				break
			}
		}
	case !slices.Contains(statuses, found[0]):
		issue.Kind = IssueUnknownStatus
	default:
		return issues
	}
	return append(issues, issue)
}

// CleanupIssues rescans and sets the status of every fixable issue, replacing all its status tags
// (other kanban-prefixed tags are kept). The fix and board are recomputed under the metadata lock,
// so a file changed since the scan is never set to a stale status. A fix that keeps the current
// status only drops the extra tags - no event is logged and KanbanMovedAt stays.
func CleanupIssues() (CleanupResult, error) {
	issues, err := ScanIssues()
	if err != nil {
		return CleanupResult{}, err
	}

	var result CleanupResult
	for _, issue := range issues {
		if issue.Fix == "" {
			continue
		}
		var oldStatus string
		var applied bool
		now := time.Now()
		err := files.MetaDataMutate(pathutils.ToWithPrefix(issue.Path), func(meta *files.Metadata, existed bool) (bool, error) {
			if !existed {
				return false, nil
			}
			dir := strings.Join(meta.Folders, "/")
			issue.Fix = ""
			for _, i := range fileIssues(issue.Path, dir, meta.Tags) {
				if i.Fix != "" {
					issue.Fix = i.Fix
				}
			}
			if issue.Fix == "" {
				return false, nil // fixed or changed since the scan
			}
			issue.Board = resolveBoard(dir).FolderPath
			oldStatus = applyStatusTag(meta, issue.Fix)
			if oldStatus != issue.Fix {
				if meta.KanbanAddedAt.IsZero() {
					meta.KanbanAddedAt = now
				}
				meta.KanbanMovedAt = now
			}
			applied = true
			return true, nil
		})
		if err != nil {
			logging.LogError(logging.KeyKanbanCleanup, "failed to set status %s for %s: %v", issue.Fix, issue.Path, err)
			result.Failed++
			continue
		}
		if !applied {
			continue
		}
		if oldStatus != issue.Fix {
			if err := kanbanStorage.LogEvent(issue.Path, issue.Board, oldStatus, issue.Fix); err != nil {
				logging.LogWarning(logging.KeyKanbanCleanup, "failed to log event for %s: %v", issue.Path, err)
			}
		}
		logging.LogInfo(logging.KeyKanbanCleanup, "set status %s for %s", issue.Fix, issue.Path)
		result.Fixed++
	}

	if result.Fixed > 0 {
		files.RefreshCaches()
	}
	return result, nil
}
